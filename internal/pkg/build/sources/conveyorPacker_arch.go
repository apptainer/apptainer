// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
)

const (
	defaultPacmanConfURL = "https://github.com/archlinux/svntogit-packages/raw/master/pacman/trunk/pacman.conf"
)

// Default list of packages to install when bootstrapping arch
// As of 2019-10-06 there is a base metapackage instead of a base group
// https://www.archlinux.org/news/base-group-replaced-by-mandatory-base-package-manual-intervention-required/
var instList = []string{"base"}

// ArchConveyorPacker only needs to hold the conveyor to have the needed data to pack
type ArchConveyorPacker struct {
	b       *types.Bundle
	confurl string
}

// prepareFakerootEnv prepares a build environment to
// make fakeroot working with pacstrap.
func (cp *ArchConveyorPacker) prepareFakerootEnv() (func(), error) {
	truePath, err := bin.FindBin("true")
	if err != nil {
		return nil, fmt.Errorf("while searching true command: %s", err)
	}
	mountPath, err := bin.FindBin("mount")
	if err != nil {
		return nil, fmt.Errorf("while searching mount command: %s", err)
	}
	umountPath, err := bin.FindBin("umount")
	if err != nil {
		return nil, fmt.Errorf("while searching umount command: %s", err)
	}

	devs := []string{
		"/dev/null",
		"/dev/random",
		"/dev/urandom",
		"/dev/zero",
	}

	devPath := filepath.Join(cp.b.RootfsPath, "dev")
	if err := os.Mkdir(devPath, 0o755); err != nil {
		return nil, fmt.Errorf("while creating %s: %s", devPath, err)
	}
	procPath := filepath.Join(cp.b.RootfsPath, "proc")
	if err := os.Mkdir(procPath, 0o755); err != nil {
		return nil, fmt.Errorf("while creating %s: %s", procPath, err)
	}

	umountFn := func() {
		syscall.Unmount(mountPath, syscall.MNT_DETACH)
		syscall.Unmount(umountPath, syscall.MNT_DETACH)
		syscall.Unmount(procPath, syscall.MNT_DETACH)
		for _, d := range devs {
			path := filepath.Join(cp.b.RootfsPath, d)
			syscall.Unmount(path, syscall.MNT_DETACH)
		}
	}

	// bind /bin/true on top of mount/umount command so
	// pacstrap wouldn't fail while preparing chroot
	// environment
	if err := syscall.Mount(truePath, mountPath, "", syscall.MS_BIND, ""); err != nil {
		return umountFn, fmt.Errorf("while mounting %s to %s: %s", truePath, mountPath, err)
	}
	if err := syscall.Mount(truePath, umountPath, "", syscall.MS_BIND, ""); err != nil {
		return umountFn, fmt.Errorf("while mounting %s to %s: %s", truePath, umountPath, err)
	}
	if err := syscall.Mount("/proc", procPath, "", syscall.MS_BIND, ""); err != nil {
		return umountFn, fmt.Errorf("while mounting /proc to %s: %s", procPath, err)
	}

	// mount required block devices
	for _, p := range devs {
		rootfsPath := filepath.Join(cp.b.RootfsPath, p)
		if err := fs.Touch(rootfsPath); err != nil {
			return umountFn, fmt.Errorf("while creating %s: %s", rootfsPath, err)
		}
		if err := syscall.Mount(p, rootfsPath, "", syscall.MS_BIND, ""); err != nil {
			return umountFn, fmt.Errorf("while mounting %s to %s: %s", p, rootfsPath, err)
		}
	}

	return umountFn, nil
}

// Get just stores the source
//
// FIXME: use context for cancellation.
func (cp *ArchConveyorPacker) Get(_ context.Context, b *types.Bundle) (err error) {
	cp.b = b

	err = cp.getBootstrapOptions()
	if err != nil {
		return fmt.Errorf("while getting bootstrap options: %v", err)
	}

	// check for pacstrap on system
	pacstrapPath, err := bin.FindBin("pacstrap")
	if err != nil {
		return fmt.Errorf("pacstrap is not in PATH: %v", err)
	}

	// make sure architecture is supported
	if arch := runtime.GOARCH; arch != `amd64` {
		return fmt.Errorf("%v architecture is not supported", arch)
	}

	pacConf, err := cp.getPacConf(cp.confurl)
	if err != nil {
		return fmt.Errorf("while getting pacman config: %v", err)
	}

	insideUserNs, setgroupsAllowed := namespaces.IsInsideUserNamespace(os.Getpid())
	if insideUserNs && setgroupsAllowed {
		umountFn, err := cp.prepareFakerootEnv()
		if umountFn != nil {
			defer umountFn()
		}
		if err != nil {
			return fmt.Errorf("while preparing fakeroot build environment: %s", err)
		}
	}

	args := []string{"-C", pacConf, "-c", "-G", "-M", cp.b.RootfsPath, "haveged"}
	args = append(args, instList...)

	pacCmd := exec.Command(pacstrapPath, args...)
	pacCmd.Stdout = os.Stdout
	pacCmd.Stderr = os.Stderr
	sylog.Debugf("\n\tPacstrap Path: %s\n\tPac Conf: %s\n\tRootfs: %s\n\tInstall List: %s\n", pacstrapPath, pacConf, cp.b.RootfsPath, instList)

	if err = pacCmd.Run(); err != nil {
		return fmt.Errorf("while pacstrapping: %v", err)
	}

	// Pacman package signing setup
	cmd := exec.Command("arch-chroot", cp.b.RootfsPath, "/bin/sh", "-c", "haveged -w 1024; pacman-key --init; pacman-key --populate archlinux")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("while setting up package signing: %v", err)
	}

	// Clean up haveged
	cmd = exec.Command("arch-chroot", cp.b.RootfsPath, "pacman", "-Rs", "--noconfirm", "haveged")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("while cleaning up packages: %v", err)
	}

	return nil
}

// Pack puts relevant objects in a Bundle!
func (cp *ArchConveyorPacker) Pack(context.Context) (b *types.Bundle, err error) {
	err = cp.insertBaseEnv()
	if err != nil {
		return nil, fmt.Errorf("while inserting base environment: %v", err)
	}

	err = cp.insertRunScript()
	if err != nil {
		return nil, fmt.Errorf("while inserting runscript: %v", err)
	}

	return cp.b, nil
}

func (cp *ArchConveyorPacker) getBootstrapOptions() (err error) {
	var include string
	var ok bool

	// get ConfURL component to definition
	cp.confurl, ok = cp.b.Recipe.Header["confurl"]
	if !ok {
		cp.confurl = defaultPacmanConfURL
	}

	// get Include component to instList
	include, ok = cp.b.Recipe.Header["include"]
	if ok {
		// trim leading and trailing whitespace
		include = strings.TrimSpace(include)

		instList = []string{include}
	}

	return nil
}

func (cp *ArchConveyorPacker) getPacConf(pacmanConfURL string) (pacConf string, err error) {
	pacConfFile, err := os.CreateTemp(cp.b.RootfsPath, "pac-conf-")
	if err != nil {
		return
	}

	resp, err := http.Get(pacmanConfURL)
	if err != nil {
		return "", fmt.Errorf("while performing http request: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(pacConfFile, resp.Body)
	if err != nil {
		return
	}

	return pacConfFile.Name(), nil
}

func (cp *ArchConveyorPacker) insertBaseEnv() (err error) {
	if err = makeBaseEnv(cp.b.RootfsPath); err != nil {
		return
	}
	return nil
}

func (cp *ArchConveyorPacker) insertRunScript() (err error) {
	err = os.WriteFile(filepath.Join(cp.b.RootfsPath, "/.singularity.d/runscript"), []byte("#!/bin/sh\n"), 0o755)
	if err != nil {
		return
	}

	return nil
}

// CleanUp removes any tmpfs owned by the conveyorPacker on the filesystem
func (cp *ArchConveyorPacker) CleanUp() {
	cp.b.Remove()
}
