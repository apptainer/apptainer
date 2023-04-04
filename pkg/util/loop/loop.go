// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
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
	"time"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/fs/lock"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	"golang.org/x/sys/unix"
)

// Device describes a loop device
type Device struct {
	MaxLoopDevices int
	Shared         bool
	Info           *unix.LoopInfo64
	fd             *int
}

// Loop control device IOCTL commands
const (
	CmdCtlAdd     = 0x4C80
	CmdCtlRemove  = 0x4C81
	CmdCtlGetFree = 0x4C82
)

// loop status retry related constants
const (
	sleepRetries      = 5
	sleepInterval     = 200 * time.Millisecond
	flushGracePeriod  = 1 * time.Second
	errStatusTryAgain = syscall.EAGAIN
)

// loop status retry function.
type retryStatusFn func(string, int) error

const (
	loopControlPath = "/dev/loop-control"
)

// create loop device function.
type createDeviceFn func(int) error

// AttachFromFile attempts to find a suitable loop device to use for the specified image.
// It runs through /dev/loopXX, up to MaxLoopDevices to find a free loop device, or
// to share a loop device already associated to file (if shared loop devices are enabled).
// If a usable loop device is found, then loop.Fd is set and no error is returned.
// If a usable loop device is not found, and this is due to a transient EAGAIN / EBUSY error,
// then it will retry up to maxRetries times, retryInterval apart, before returning an error.
func (loop *Device) AttachFromFile(image *os.File, mode int, number *int) error {
	if image == nil {
		return fmt.Errorf("empty file pointer")
	}
	fi, err := image.Stat()
	if err != nil {
		return err
	}
	imageInfo := fi.Sys().(*syscall.Stat_t)

	if loop.Shared {
		if ok, err := loop.shareLoop(imageInfo, mode, number); err != nil {
			return err
		} else if ok {
			// We found a shared loop device, and loop.Fd was set
			return nil
		}
	}

	if err := loop.attachLoop(image.Fd(), imageInfo, mode, number); err != nil {
		return fmt.Errorf("failed to attach loop device: %s", err)
	}

	return nil
}

// shareLoop runs over /dev/loopXX devices, looking for one that already has our image attached.
// If a loop device can be shared, loop.Fd is set, and ok will be true.
// If no loop device can be shared, ok will be false.
func (loop *Device) shareLoop(imageInfo *syscall.Stat_t, mode int, number *int) (ok bool, err error) {
	imageIno := imageInfo.Ino
	// cast to uint64 as st.Dev is uint32 on MIPS
	imageDev := uint64(imageInfo.Dev)

	for device := 0; device < loop.MaxLoopDevices; device++ {
		// Try to open an existing loop device, but don't create a new one
		loopFd, releaseLock, err := openLoopDev(device, mode, true, nil)
		if err != nil {
			if !os.IsNotExist(err) {
				sylog.Debugf("Couldn't open loop device %d: %s\n", device, err)
			}
			continue
		}

		status, err := GetStatusFromFd(uintptr(loopFd))
		releaseLock()
		if err != nil {
			sylog.Debugf("Couldn't get status from loop device %d: %v\n", device, err)
		} else if status.Inode == imageIno && status.Device == imageDev &&
			status.Flags&unix.LO_FLAGS_READ_ONLY == loop.Info.Flags&unix.LO_FLAGS_READ_ONLY &&
			status.Offset == loop.Info.Offset && status.Sizelimit == loop.Info.Sizelimit {
			// keep the reference to the loop device file descriptor to
			// be sure that the loop device won't be released between this
			// check and the mount of the filesystem
			sylog.Debugf("Sharing loop device %d", device)
			*number = device
			loop.fd = &loopFd
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
func (loop *Device) attachLoop(imageFd uintptr, imageInfo *syscall.Stat_t, mode int, number *int) error {
	releaseDevice := func(fd int, clear bool, releaseLock func()) {
		if clear {
			unix.IoctlSetInt(fd, unix.LOOP_CLR_FD, 0)
		}
		syscall.Close(fd)
		releaseLock()
	}

	createFn := getCreateDeviceFn()
	retryFn := getRetryStatusFn(imageFd, imageInfo)

	for device := 0; device < loop.MaxLoopDevices; device++ {
		// Try to open the loop device, creating the device node if needed
		loopFd, releaseLock, err := openLoopDev(device, mode, loop.Shared, createFn)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			sylog.Debugf("Couldn't open loop device %d: %v", device, err)
			return err
		}

		if err := unix.IoctlSetInt(loopFd, unix.LOOP_SET_FD, int(imageFd)); err != nil {
			// On error, we'll move on to try the next loop device
			releaseDevice(loopFd, false, releaseLock)
			continue
		}

		if _, _, esys := syscall.Syscall(syscall.SYS_FCNTL, uintptr(loopFd), syscall.F_SETFD, syscall.FD_CLOEXEC); esys != 0 {
			releaseDevice(loopFd, true, releaseLock)
			return fmt.Errorf("failed to set close-on-exec on loop device %s: error message=%s", getLoopPath(device), esys.Error())
		}

		if err := setLoopStatus(loopFd, loop.Info, getLoopPath(device), retryFn); err != nil {
			releaseDevice(loopFd, true, releaseLock)
			return fmt.Errorf("loop device status: %s", err)
		}

		releaseLock()
		*number = device
		loop.fd = &loopFd
		return nil
	}

	return fmt.Errorf("no loop devices available")
}

// openLoopDev will attempt to open the specified loop device number, with specified mode.
// If it is not present in /dev, and create is true, a mknod call will be used to create it.
// Returns the fd for the opened device, or -1 if it was not possible to openLoopDev it.
func openLoopDev(device, mode int, sharedLoop bool, createFn createDeviceFn) (int, func(), error) {
	path := getLoopPath(device)

	// loop device can exist but without any device attached to it in kernel,
	// a stat call couldn't catch ENXIO error in this case, use open
	loopFd, err := syscall.Open(path, mode, 0o600)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == unix.ENXIO {
			if createFn == nil {
				err = os.ErrNotExist
			}
		} else if !os.IsNotExist(err) {
			return -1, nil, fmt.Errorf("could not open %s: %w", path, err)
		}
		// device doesn't exist but no create function passed ... done
		if createFn == nil {
			return -1, nil, err
		}
		// create the device node if we need to
		err := createFn(device)
		if err != nil {
			return -1, nil, fmt.Errorf("could not create %s: %w", path, err)
		}
	} else {
		_ = syscall.Close(loopFd)
	}

	releaseLock := func() {}

	if sharedLoop {
		// there is an exclusive lock set on the opened loop device
		// when shared loop devices is in-use, this lock is intended
		// to be hold until the loop device status is set for the
		// opened loop device
		loopLock, err := lock.Exclusive(path)
		if err != nil {
			return -1, nil, fmt.Errorf("while acquiring exclusive lock on %s: %s", path, err)
		}
		releaseLock = func() {
			_ = lock.Release(loopLock)
		}
	}

	loopFd, err = syscall.Open(path, mode, 0o600)
	if err != nil {
		releaseLock()
		return -1, nil, fmt.Errorf("could not open %s: %w", path, err)
	}

	return loopFd, releaseLock, nil
}

func setLoopStatus(loopFd int, info *unix.LoopInfo64, loopDevice string, retryFn retryStatusFn) error {
	for retryCount := 0; ; retryCount++ {
		esys := unix.IoctlLoopSetStatus64(loopFd, info)
		if esys == nil {
			return nil
		} else if esys != syscall.EAGAIN {
			return fmt.Errorf("failed to set loop device status (%s): %s", loopDevice, esys)
		}

		if err := retryFn(loopDevice, retryCount); err != errStatusTryAgain {
			return err
		}
	}
}

func getRetryStatusFn(imageFd uintptr, imageInfo *syscall.Stat_t) retryStatusFn {
	return func(loopDevice string, retryCount int) error {
		// With changes introduced in https://github.com/torvalds/linux/commit/5db470e229e22b7eda6e23b5566e532c96fb5bc3
		// loop device is invalidating its cache when offset/sizelimit are modified while issuing the set status command,
		// as there is no synchronization between the invalidation and the check for cached dirty pages, some kernel may
		// return an EAGAIN error here. Note that this error is occurring very frequently with small images.
		// The first approach is to sleep and retry, the problem is that the underlying filesystem backing the image file
		// may be slow, so setting a time interval and number of retries may be hazardous, and trying other loop devices
		// just deport the issue to the next devices as falsely stated here https://dev.arvados.org/issues/18489.
		// So retry 5 times with a sleep period of 200ms between each attempt.
		if retryCount < sleepRetries {
			time.Sleep(sleepInterval)
			return errStatusTryAgain
		} else if retryCount == sleepRetries {
			// The sleeping period is over and there is remaining dirty pages in cache for the corresponding image,
			// let's use the rough approach to flush cached pages to filesystem, the kernel is not providing a way to
			// flush and wait, syncfs/fsync/sync_file_range are not working as expected here, so we call flushCache
			// which will try to issue a block device flush command when the image is located on a block device, if the
			// image is on a shared storage, a ramfs or anything else which isn't a block device, a sync syscall is
			// issued with the costs it involved.
			//
			// Dear reader, if you are not satisfied by the approach, you are invited to reproduce the issue first by using
			// an Ubuntu 18.04 distribution containing the fix/bug above and build a small image like:
			//
			// $ apptainer build /tmp/busy.sif docker://busybox
			// $ for i in $(seq 1 100); do apptainer exec /tmp/busy.sif true; done
			//
			// And search a more elegant solution
			if err := flushCache(imageFd, imageInfo); err != nil {
				return fmt.Errorf("while syncing/flushing image cache: %s", err)
			}
			return errStatusTryAgain
		} else if retryCount == sleepRetries+1 {
			// e2e tests have shown that the sync approach is not sufficient under high load
			// circumstances, therefore we are giving an additional grace period, after that
			// we are over and return a cache invalidate too slow error
			time.Sleep(flushGracePeriod)
			return errStatusTryAgain
		}

		return fmt.Errorf("failed to set loop device status (%s): cache invalidate too slow", loopDevice)
	}
}

func flushCache(imageFd uintptr, imageInfo *syscall.Stat_t) error {
	devStr := fmt.Sprintf("%d:%d", unix.Major(imageInfo.Dev), unix.Minor(imageInfo.Dev))
	entries, err := proc.GetMountInfoEntry("/proc/self/mountinfo")
	if err != nil {
		return fmt.Errorf("while getting mountinfo: %s", err)
	}
	for _, e := range entries {
		if e.Dev != devStr {
			continue
		}
		fi, err := os.Stat(e.Source)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("while getting information for %s: %s", e.Source, err)
		}
		// not a block device
		if fi.Mode()&os.ModeDevice == 0 {
			continue
		}
		// trigger block device flush command
		f, err := os.Open(e.Source)
		if err != nil {
			return fmt.Errorf("while opening %s: %s", e.Source, err)
		}
		defer f.Close()

		_, _, esys := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), unix.BLKFLSBUF, 0)
		if esys != 0 {
			return fmt.Errorf("while flushing block device %s: %s", e.Source, syscall.Errno(esys))
		}

		return nil
	}
	// use sync as a last resort
	unix.Sync()
	return nil
}

func getCreateDeviceFn() createDeviceFn {
	return func(device int) error {
		path := getLoopPath(device)
		// use /dev/loop-control when possible
		controlFd, err := syscall.Open(loopControlPath, syscall.O_RDWR, 0o600)
		if err != nil {
			// create loop device with mknod as a fallback
			return createLoopDevice(device)
		}
		defer syscall.Close(controlFd)

		// use an exclusive lock on /dev/loop-control
		// mainly to prevent race conditions with other
		// instances while issuing LOOP_CTL_REMOVE command
		loopControlLock, err := lock.Exclusive(loopControlPath)
		if err != nil {
			return fmt.Errorf("while acquiring exclusive lock on %s: %w", loopControlPath, err)
		}
		defer lock.Release(loopControlLock)

		for try := 0; ; try++ {
			// issue a LOOP_CTL_ADD to add the corresponding loop device
			devNum, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(controlFd), CmdCtlAdd, uintptr(device))
			if errno > 0 && errno != syscall.EEXIST {
				return fmt.Errorf("could not add device %s: %w", path, errno)
			} else if int(devNum) == device {
				if _, err := os.Stat(path); err != nil {
					if os.IsNotExist(err) {
						// handle docker container case where /dev/loop-control is available
						// but loop devices are created on /dev host, so create it in container
						return createLoopDevice(device)
					}
					return fmt.Errorf("while retrieving %s status: %s", path, err)
				}
				break
			}
			// handle a corner case where the device hasn't been created,
			// it might happen when a /dev/loopX is deleted with rm /dev/loopX
			// without issuing a LOOP_CTL_REMOVE for the corresponding device
			_, err := os.Stat(path)
			if err != nil && try == 0 {
				// issue a LOOP_CTL_REMOVE to remove the corresponding loop device in kernel
				_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(controlFd), CmdCtlRemove, uintptr(device))
				if errno > 0 {
					if errno == syscall.EBUSY {
						break
					}
					return fmt.Errorf("could not remove device %s: %w", path, errno)
				}
				// and retry to add the loop device
				continue
			} else if err != nil && try == 1 {
				return fmt.Errorf("could not add device %s: %w", path, err)
			}
			break
		}

		return nil
	}
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
func GetStatusFromFd(fd uintptr) (*unix.LoopInfo64, error) {
	info, err := unix.IoctlLoopGetStatus64(int(fd))
	if err != nil {
		return nil, fmt.Errorf("failed to get loop flags for loop device: %s", err)
	}
	return info, nil
}

// GetStatusFromPath gets info status about a loop device from path
func GetStatusFromPath(path string) (*unix.LoopInfo64, error) {
	loop, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open loop device %s: %s", path, err)
	}
	return GetStatusFromFd(loop.Fd())
}

func getLoopPath(device int) string {
	return fmt.Sprintf("/dev/loop%d", device)
}

func createLoopDevice(device int) error {
	// create loop device with mknod as a fallback
	dev := int(unix.Mkdev(uint32(7), uint32(device)))
	path := getLoopPath(device)
	esys := syscall.Mknod(path, syscall.S_IFBLK|0o660, dev)
	if errno, ok := esys.(syscall.Errno); ok {
		if errno != syscall.EEXIST {
			return fmt.Errorf("could not mknod %s: %w", path, esys)
		}
	}
	return nil
}
