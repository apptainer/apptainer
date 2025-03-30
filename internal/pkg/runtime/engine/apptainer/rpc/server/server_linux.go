// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package server

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	args "github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer/rpc"
	"github.com/apptainer/apptainer/internal/pkg/util/crypt"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/gpu"
	"github.com/apptainer/apptainer/internal/pkg/util/mainthread"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/loop"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
	"golang.org/x/sys/unix"
)

var (
	diskGID          = -1
	defaultEffective = uint64(0)
)

func init() {
	defaultEffective |= uint64(1 << capabilities.Map["CAP_SETUID"].Value)
	defaultEffective |= uint64(1 << capabilities.Map["CAP_SETGID"].Value)
	defaultEffective |= uint64(1 << capabilities.Map["CAP_SYS_ADMIN"].Value)
}

// Methods is a receiver type.
type Methods int

// Mount performs a mount with the specified arguments.
func (t *Methods) Mount(arguments *args.MountArgs, mountErr *error) (err error) {
	mainthread.Execute(func() {
		if arguments.Filesystem == "overlay" {
			var oldEffective uint64

			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			caps := uint64(0)
			caps |= uint64(1 << capabilities.Map["CAP_FOWNER"].Value)
			caps |= uint64(1 << capabilities.Map["CAP_DAC_OVERRIDE"].Value)
			caps |= uint64(1 << capabilities.Map["CAP_DAC_READ_SEARCH"].Value)
			caps |= uint64(1 << capabilities.Map["CAP_CHOWN"].Value)
			caps |= uint64(1 << capabilities.Map["CAP_SYS_ADMIN"].Value)

			oldEffective, err = capabilities.SetProcessEffective(caps)
			if err != nil {
				return
			}
			defer func() {
				_, err = capabilities.SetProcessEffective(oldEffective)
			}()
		}
		*mountErr = syscall.Mount(arguments.Source, arguments.Target, arguments.Filesystem, arguments.Mountflags, arguments.Data)
	})
	return
}

// Unmount performs an unmount with the specified arguments.
func (t *Methods) Unmount(arguments *args.UnmountArgs, unmountErr *error) (err error) {
	mainthread.Execute(func() {
		*unmountErr = syscall.Unmount(arguments.Target, arguments.Unmountflags)
	})
	return
}

// Decrypt decrypts the loop device.
func (t *Methods) Decrypt(arguments *args.CryptArgs, reply *string) (err error) {
	cryptName := ""
	cryptDev := &crypt.Device{}
	hasIPC := arguments.MasterPid > 0

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	caps := defaultEffective
	caps |= uint64(1 << capabilities.Map["CAP_IPC_LOCK"].Value)

	if hasIPC {
		// required for /proc/pid/ns/ipc access with namespaces.Enter
		caps |= uint64(1 << capabilities.Map["CAP_SYS_PTRACE"].Value)
	}

	oldEffective, capErr := capabilities.SetProcessEffective(caps)
	if capErr != nil {
		return capErr
	}

	pid := 0

	if hasIPC {
		// we can't rely on os.Getpid since this process may
		// be in the container PID namespace
		self, err := os.Readlink("/proc/self")
		if err != nil {
			return err
		}
		pid, err = strconv.Atoi(self)
		if err != nil {
			return err
		}

		// cryptsetup requires to run in the host IPC namespace
		// so we enter temporarily in the host IPC namespace
		// via the master process ID if its greater than zero
		// which means that a container IPC namespace was requested
		if err := namespaces.Enter(arguments.MasterPid, "ipc"); err != nil {
			return fmt.Errorf("while joining host IPC namespace: %s", err)
		}
	}

	defer func() {
		_, e := capabilities.SetProcessEffective(oldEffective)
		if err == nil {
			err = e
		}

		if hasIPC {
			e := namespaces.Enter(pid, "ipc")
			if err == nil && e != nil {
				err = fmt.Errorf("while joining container IPC namespace: %s", e)
			}
		}
	}()

	cryptName, err = cryptDev.Open(arguments.Key, arguments.Loopdev)

	*reply = "/dev/mapper/" + cryptName

	return err
}

// Mkdir performs a mkdir with the specified arguments.
func (t *Methods) Mkdir(arguments *args.MkdirArgs, _ *int) (err error) {
	mainthread.Execute(func() {
		oldmask := syscall.Umask(0)
		err = os.Mkdir(arguments.Path, arguments.Perm)
		syscall.Umask(oldmask)
	})
	return err
}

// Chroot performs a chroot with the specified arguments.
func (t *Methods) Chroot(arguments *args.ChrootArgs, _ *int) (err error) {
	root := arguments.Root

	if root != "." {
		sylog.Debugf("Change current directory to %s", root)
		if err := syscall.Chdir(root); err != nil {
			return fmt.Errorf("failed to change directory to %s", root)
		}
	} else {
		cwd, err := os.Getwd()
		if err == nil {
			root = cwd
		}
	}

	var oldEffective uint64

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	caps := uint64(0)
	caps |= uint64(1 << capabilities.Map["CAP_SYS_CHROOT"].Value)
	caps |= uint64(1 << capabilities.Map["CAP_SYS_ADMIN"].Value)

	oldEffective, err = capabilities.SetProcessEffective(caps)
	if err != nil {
		return err
	}
	defer func() {
		_, e := capabilities.SetProcessEffective(oldEffective)
		if err == nil {
			err = e
		}
	}()

	switch arguments.Method {
	case "pivot":
		// idea taken from libcontainer (and also LXC developers) to avoid
		// creation of temporary directory or use of existing directory
		// for pivot_root.

		sylog.Debugf("Hold reference to host / directory")
		oldroot, err := os.Open("/")
		if err != nil {
			return fmt.Errorf("failed to open host root directory: %s", err)
		}
		defer oldroot.Close()

		sylog.Debugf("Called pivot_root on %s\n", root)
		if err := syscall.PivotRoot(".", "."); err != nil {
			return fmt.Errorf("pivot_root %s: %s", root, err)
		}

		sylog.Debugf("Change current directory to host / directory")
		if err := syscall.Fchdir(int(oldroot.Fd())); err != nil {
			return fmt.Errorf("failed to change directory to old root: %s", err)
		}

		sylog.Debugf("Apply slave mount propagation for host / directory")
		if err := syscall.Mount("", ".", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("failed to apply slave mount propagation for host / directory: %s", err)
		}

		sylog.Debugf("Called unmount(/, syscall.MNT_DETACH)\n")
		if err := syscall.Unmount(".", syscall.MNT_DETACH); err != nil {
			return fmt.Errorf("unmount pivot_root dir %s", err)
		}
	case "move":
		sylog.Debugf("Move %s as / directory", root)
		if err := syscall.Mount(".", "/", "", syscall.MS_MOVE, ""); err != nil {
			return fmt.Errorf("failed to move %s as / directory: %s", root, err)
		}

		sylog.Debugf("Chroot to %s", root)
		if err := syscall.Chroot("."); err != nil {
			return fmt.Errorf("chroot failed: %s", err)
		}
	case "chroot":
		sylog.Debugf("Chroot to %s", root)
		if err := syscall.Chroot("."); err != nil {
			return fmt.Errorf("chroot failed: %s", err)
		}
	}

	sylog.Debugf("Changing directory to / to avoid getpwd issues\n")
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %s", err)
	}

	return err
}

// LoopDevice attaches a loop device with the specified arguments.
func (t *Methods) LoopDevice(arguments *args.LoopArgs, reply *int) (err error) {
	var image *os.File

	loopdev := &loop.Device{}
	loopdev.MaxLoopDevices = arguments.MaxDevices
	loopdev.Info = &arguments.Info
	loopdev.Shared = arguments.Shared

	if strings.HasPrefix(arguments.Image, "/proc/self/fd/") {
		strFd := strings.TrimPrefix(arguments.Image, "/proc/self/fd/")
		fd, err := strconv.ParseUint(strFd, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to convert image file descriptor: %v", err)
		}
		image = os.NewFile(uintptr(fd), "")
	} else {
		var err error
		image, err = os.OpenFile(arguments.Image, arguments.Mode, 0o600)
		if err != nil {
			return fmt.Errorf("could not open image file: %v", err)
		}
	}

	if diskGID == -1 {
		if gr, err := user.GetGrNam("disk"); err == nil {
			diskGID = int(gr.GID)
		} else {
			diskGID = 0
		}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	caps := defaultEffective
	caps |= uint64(1 << capabilities.Map["CAP_MKNOD"].Value)

	oldEffective, err := capabilities.SetProcessEffective(caps)
	if err != nil {
		return err
	}

	syscall.Setfsuid(0)
	syscall.Setfsgid(diskGID)

	defer func() {
		syscall.Setfsuid(os.Getuid())
		syscall.Setfsgid(os.Getgid())

		_, e := capabilities.SetProcessEffective(oldEffective)
		if err == nil {
			err = e
		}
	}()

	if err := loopdev.AttachFromFile(image, arguments.Mode, reply); err != nil {
		return fmt.Errorf("could not attach image file to loop device: %v", err)
	}

	return err
}

// SetHostname sets hostname with the specified arguments.
func (t *Methods) SetHostname(arguments *args.HostnameArgs, _ *int) error {
	return syscall.Sethostname([]byte(arguments.Hostname))
}

// Chdir changes current working directory to path.
func (t *Methods) Chdir(arguments *args.ChdirArgs, _ *int) error {
	return mainthread.Chdir(arguments.Dir)
}

// Stat gets file status.
func (t *Methods) Stat(arguments *args.StatArgs, reply *args.StatReply) error {
	reply.Fi, reply.Err = os.Stat(arguments.Path)
	if reply.Fi != nil {
		reply.Fi = args.FileInfo(reply.Fi)
	}
	return nil
}

// Lstat gets file status.
func (t *Methods) Lstat(arguments *args.StatArgs, reply *args.StatReply) error {
	reply.Fi, reply.Err = os.Lstat(arguments.Path)
	if reply.Fi != nil {
		reply.Fi = args.FileInfo(reply.Fi)
	}
	return nil
}

// Access checks file access permissions
func (t *Methods) Access(arguments *args.AccessArgs, reply *args.AccessReply) error {
	reply.Err = syscall.Access(arguments.Path, arguments.Mode)
	return nil
}

// SendFuseFd send fuse file descriptor over unix socket.
func (t *Methods) SendFuseFd(arguments *args.SendFuseFdArgs, _ *int) error {
	usernsFd, err := unix.Open("/proc/self/ns/user", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("while opening /proc/self/ns/user: %s", err)
	}
	defer unix.Close(usernsFd)

	rights := unix.UnixRights(append(arguments.Fds, usernsFd)...)
	// The second parameter here was added as a workaround after
	//  the following change to golang.org/x/sys/unix which removed
	//  that value as a default:
	//     https://go-review.googlesource.com/c/sys/+/412497
	err = unix.Sendmsg(arguments.Socket, []byte{0}, rights, nil, 0)
	return err
}

// OpenSendFuseFd open a new /dev/fuse file descriptor and send it
// over unix socket.
func (t *Methods) OpenSendFuseFd(arguments *args.OpenSendFuseFdArgs, reply *int) error {
	fd, err := unix.Open("/dev/fuse", unix.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("while opening /dev/fuse: %s", err)
	}
	*reply = fd

	rights := unix.UnixRights(fd)
	return unix.Sendmsg(arguments.Socket, []byte{0}, rights, nil, 0)
}

// Symlink performs a symlink with the specified arguments.
func (t *Methods) Symlink(arguments *args.SymlinkArgs, _ *int) error {
	return os.Symlink(arguments.Target, arguments.Link)
}

// ReadDir performs a readdir with the specified arguments.
func (t *Methods) ReadDir(arguments *args.ReadDirArgs, reply *args.ReadDirReply) error {
	files, err := os.ReadDir(arguments.Dir)
	if err != nil {
		return err
	}

	for i, file := range files {
		files[i], err = args.DirEntry(file)
		if err != nil {
			return err
		}
	}

	reply.Files = files
	return nil
}

// Chown performs a chown with the specified arguments.
func (t *Methods) Chown(arguments *args.ChownArgs, _ *int) error {
	return os.Chown(arguments.Name, arguments.UID, arguments.GID)
}

// Lchown performs a lchown with the specified arguments.
func (t *Methods) Lchown(arguments *args.ChownArgs, _ *int) error {
	return os.Lchown(arguments.Name, arguments.UID, arguments.GID)
}

// EvalRelative calls EvalRelative with the specified arguments.
func (t *Methods) EvalRelative(arguments *args.EvalRelativeArgs, reply *string) error {
	*reply = fs.EvalRelative(arguments.Name, arguments.Root)
	return nil
}

// Readlink performs a readlink with the specified arguments.
func (t *Methods) Readlink(arguments *args.ReadlinkArgs, reply *string) (err error) {
	*reply, err = os.Readlink(arguments.Name)
	return err
}

// Umask performs a umask with the specified arguments.
func (t *Methods) Umask(arguments *args.UmaskArgs, reply *int) (err error) {
	*reply = syscall.Umask(arguments.Mask)
	return nil
}

// WriteFile creates an empty file if it doesn't exist or a file with the provided data.
func (t *Methods) WriteFile(arguments *args.WriteFileArgs, _ *int) error {
	f, err := os.OpenFile(arguments.Filename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, arguments.Perm)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create file %s: %s", arguments.Filename, err)
		}
		return err
	}
	if len(arguments.Data) > 0 {
		_, err = f.Write(arguments.Data)
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

// NvCCLI will call nvidia-container-cli to configure GPU(s) for the container.
func (t *Methods) NvCCLI(arguments *args.NvCCLIArgs, _ *int) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// In the setuid flow we need CAP_CHOWN here to be able to start
	// nvidia-container-cli successfully as root.
	caps := defaultEffective
	if !arguments.UserNS {
		caps |= uint64(1 << capabilities.Map["CAP_CHOWN"].Value)
	}
	oldEffective, err := capabilities.SetProcessEffective(caps)
	if err != nil {
		return err
	}
	defer func() {
		_, e := capabilities.SetProcessEffective(oldEffective)
		if err == nil {
			err = e
		}
	}()

	return gpu.NVCLIConfigure(arguments.Flags, arguments.RootFsPath, arguments.UserNS)
}
