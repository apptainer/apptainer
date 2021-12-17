// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// Copyright (c) 2021, Genomics plc.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package loop

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/fs/lock"
	"golang.org/x/sys/unix"
)

// Device describes a loop device
type Device struct {
	MaxLoopDevices int
	Shared         bool
	Info           *Info64
	fd             *int
}

// Loop device flags values
const (
	FlagsReadOnly  = 1
	FlagsAutoClear = 4
	FlagsPartScan  = 8
	FlagsDirectIO  = 16
)

// Loop device encryption types
const (
	CryptNone      = 0
	CryptXor       = 1
	CryptDes       = 2
	CryptFish2     = 3
	CryptBlow      = 4
	CryptCast128   = 5
	CryptIdea      = 6
	CryptDummy     = 9
	CryptSkipJack  = 10
	CryptCryptoAPI = 18
	CryptMax       = 20
)

// Loop device IOCTL commands
const (
	CmdSetFd       = 0x4C00
	CmdClrFd       = 0x4C01
	CmdSetStatus   = 0x4C02
	CmdGetStatus   = 0x4C03
	CmdSetStatus64 = 0x4C04
	CmdGetStatus64 = 0x4C05
	CmdChangeFd    = 0x4C06
	CmdSetCapacity = 0x4C07
	CmdSetDirectIO = 0x4C08
)

// Info64 contains information about a loop device.
type Info64 struct {
	Device         uint64
	Inode          uint64
	Rdevice        uint64
	Offset         uint64
	SizeLimit      uint64
	Number         uint32
	EncryptType    uint32
	EncryptKeySize uint32
	Flags          uint32
	FileName       [64]byte
	CryptName      [64]byte
	EncryptKey     [32]byte
	Init           [2]uint64
}

// AttachFromFile attempts to find a suitable loop device to use for the specified image.
// It runs through /dev/loopXX, up to MaxLoopDevices to find a free loop device, or
// to share a loop device already associated to file (if shared loop devices are enabled).
// If a usable loop device is found, then loop.Fd is set and no error is returned.
// If a usable loop device is not found, and this is due to a transient EAGAIN / EBUSY error,
// then it will retry up to maxRetries times, retryInterval apart, before returning an error.
func (loop *Device) AttachFromFile(image *os.File, mode int, number *int) error {
	var err error

	if image == nil {
		return fmt.Errorf("empty file pointer")
	}
	fi, err := image.Stat()
	if err != nil {
		return err
	}
	st := fi.Sys().(*syscall.Stat_t)
	imageIno := st.Ino
	// cast to uint64 as st.Dev is uint32 on MIPS
	imageDev := uint64(st.Dev)

	if loop.Shared {
		if ok, err := loop.shareLoop(imageIno, imageDev, mode, number); err != nil {
			return err
		} else if ok {
			// We found a shared loop device, and loop.Fd was set
			return nil
		}
		loop.Shared = false
	}

	if err := loop.attachLoop(image, mode, number); err != nil {
		return fmt.Errorf("failed to attach loop device: %s", err)
	}

	return nil
}

// shareLoop runs over /dev/loopXX devices, looking for one that already has our image attached.
// If a loop device can be shared, loop.Fd is set, and ok will be true.
// If no loop device can be shared, ok will be false.
func (loop *Device) shareLoop(imageIno, imageDev uint64, mode int, number *int) (ok bool, err error) {
	// Because we hold a lock on /dev here, avoid delayed retries inside this function,
	// as it could impact parallel startup of many instances of Singularity or
	// other programs.
	fd, err := lock.Exclusive("/dev")
	if err != nil {
		return false, err
	}
	defer lock.Release(fd)

	for device := 0; device < loop.MaxLoopDevices; device++ {
		*number = device

		// Try to open an existing loop device, but don't create a new one
		loopFd, err := openLoopDev(device, mode, false)
		if err != nil {
			if !os.IsNotExist(err) {
				sylog.Debugf("Couldn't open loop device %d: %v\n", device, err)
			}
			continue
		}

		status, err := GetStatusFromFd(uintptr(loopFd))
		if err != nil {
			sylog.Debugf("Couldn't get status from loop device %d: %v\n", device, err)
		} else if status.Inode == imageIno && status.Device == imageDev &&
			status.Flags&FlagsReadOnly == loop.Info.Flags&FlagsReadOnly &&
			status.Offset == loop.Info.Offset && status.SizeLimit == loop.Info.SizeLimit {
			// keep the reference to the loop device file descriptor to
			// be sure that the loop device won't be released between this
			// check and the mount of the filesystem
			sylog.Debugf("Sharing loop device %d", device)
			loop.fd = new(int)
			*loop.fd = loopFd
			return true, nil
		}
		syscall.Close(loopFd)
	}

	return false, nil
}

// attachLoop will find a free /dev/loopXX device, or create a new one, and attach image to it.
// For most failures with loopN, it will try loopN+1, continuing up to loop.MaxLoopDevices.
// When setting loop device status, some kernel may return EAGAIN, this function would sync
// workaround this error.
func (loop *Device) attachLoop(image *os.File, mode int, number *int) error {
	// Because we hold a lock on /dev here, avoid delayed retries inside this function,
	// as it could impact parallel startup of many instances of Singularity or
	// other programs.
	fd, err := lock.Exclusive("/dev")
	if err != nil {
		return err
	}
	defer lock.Release(fd)

	releaseDevice := func(fd int, clear bool) {
		if clear {
			syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), CmdClrFd, 0)
		}
		syscall.Close(fd)
	}

	for device := 0; device < loop.MaxLoopDevices; device++ {
		*number = device
		clearDevice := false

		// Try to open the loop device, creating the device node if needed
		loopFd, err := openLoopDev(device, mode, true)
		if err != nil {
			sylog.Debugf("couldn't openLoopDev loop device %d: %v", device, err)
			return err
		}

		// On error, we'll move on to try the next loop device
		_, _, esys := syscall.Syscall(syscall.SYS_IOCTL, uintptr(loopFd), CmdSetFd, image.Fd())
		if esys != 0 {
			releaseDevice(loopFd, clearDevice)
			continue
		}
		clearDevice = true

		if _, _, esys := syscall.Syscall(syscall.SYS_FCNTL, uintptr(loopFd), syscall.F_SETFD, syscall.FD_CLOEXEC); esys != 0 {
			releaseDevice(loopFd, clearDevice)
			return fmt.Errorf("failed to set close-on-exec on loop device %s: %s", getLoopPath(device), err.Error())
		}

		if _, _, esys := syscall.Syscall(syscall.SYS_IOCTL, uintptr(loopFd), CmdSetStatus64, uintptr(unsafe.Pointer(loop.Info))); esys != 0 {
			if esys != syscall.EAGAIN {
				releaseDevice(loopFd, clearDevice)
				return fmt.Errorf("failed to set loop device status: %s", syscall.Errno(esys))
			}

			// With changes introduces in https://github.com/torvalds/linux/commit/5db470e229e22b7eda6e23b5566e532c96fb5bc3
			// loop device is invalidating its cache when offset/sizelimit are modified while issuing the set status command,
			// as there is no synchronization between the invalidation and the check for cached dirty pages, some kernel may
			// return an EAGAIN error here. Note that this error is occurring very frequently with small images.
			// An approach would be to sleep and retry, the problem is that the underlying filesystem backing the image file
			// may be slow, so setting a time interval and number of retries may be hazardous, and trying other loop devices
			// just deport the issue to the next devices as falsely stated here https://dev.arvados.org/issues/18489.
			// The rough approach is to trigger a sync to commit cached pages to devices, the kernel is not providing a way
			// to flush and wait for the loop devices (syncfs can't be used for loop devices), all methods seems to be
			// asynchronous leading to sleep usage, sync syscall is meeting those criteria but at some costs.
			//
			// Dear reader, if you are not satisfied by the approach, you are invited to reproduce the issue first by using
			// an Ubuntu 18.04 distribution containing the fix/bug above and build a small image like:
			//
			// $ apptainer build /tmp/busy.sif docker://busybox
			// $ for i in $(seq 1 100); do apptainer exec /tmp/busy.sif true; done
			//
			// And search a more elegant solution
			unix.Sync()

			_, _, esys := syscall.Syscall(syscall.SYS_IOCTL, uintptr(loopFd), CmdSetStatus64, uintptr(unsafe.Pointer(loop.Info)))
			if esys != 0 {
				releaseDevice(loopFd, clearDevice)
				return fmt.Errorf("failed to set loop device status: %s", syscall.Errno(esys))
			}
		}

		loop.fd = new(int)
		*loop.fd = loopFd
		return nil
	}

	return fmt.Errorf("no loop devices available")
}

// openLoopDev will attempt to open the specified loop device number, with specified mode.
// If it is not present in /dev, and create is true, a mknod call will be used to create it.
// Returns the fd for the opened device, or -1 if it was not possible to openLoopDev it.
func openLoopDev(device, mode int, create bool) (loopFd int, err error) {
	path := getLoopPath(device)
	fi, err := os.Stat(path)

	// If it doesn't exist, and create is false.. we're done..
	if os.IsNotExist(err) && !create {
		return -1, err
	}
	// If there's another stat error that's likely fatal.. we're done..
	if err != nil && !os.IsNotExist(err) {
		return -1, fmt.Errorf("could not stat %s: %w", path, err)
	}

	// Create the device node if we need to
	if os.IsNotExist(err) {
		dev := int((7 << 8) | (device & 0xff) | ((device & 0xfff00) << 12))
		esys := syscall.Mknod(path, syscall.S_IFBLK|0o660, dev)
		if errno, ok := esys.(syscall.Errno); ok {
			if errno != syscall.EEXIST {
				return -1, fmt.Errorf("could not mknod %s: %w", path, esys)
			}
		}
	} else if fi.Mode()&os.ModeDevice == 0 {
		return -1, fmt.Errorf("%s is not a block device", path)
	}

	// Now open the loop device
	loopFd, err = syscall.Open(path, mode, 0o600)
	if err != nil {
		return -1, fmt.Errorf("could not open %s: %w", path, err)
	}

	return loopFd, nil
}

// AttachFromPath finds a free loop device, opens it, and stores file descriptor
// of opened image path
func (loop *Device) AttachFromPath(image string, mode int, number *int) error {
	file, err := os.OpenFile(image, mode, 0o600)
	if err != nil {
		return err
	}
	return loop.AttachFromFile(file, mode, number)
}

// Close closes the loop device.
func (loop *Device) Close() error {
	if loop.fd != nil {
		return syscall.Close(*loop.fd)
	}
	return nil
}

// GetStatusFromFd gets info status about an opened loop device
func GetStatusFromFd(fd uintptr) (*Info64, error) {
	info := &Info64{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, CmdGetStatus64, uintptr(unsafe.Pointer(info)))
	if err != syscall.ENXIO && err != 0 {
		return nil, fmt.Errorf("failed to get loop flags for loop device: %s", err.Error())
	}
	return info, nil
}

// GetStatusFromPath gets info status about a loop device from path
func GetStatusFromPath(path string) (*Info64, error) {
	loop, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open loop device %s: %s", path, err)
	}
	return GetStatusFromFd(loop.Fd())
}

func getLoopPath(device int) string {
	return fmt.Sprintf("/dev/loop%d", device)
}
