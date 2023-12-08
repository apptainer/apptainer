// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/instance"
	fakerootConfig "github.com/apptainer/apptainer/internal/pkg/runtime/engine/fakeroot/config"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/internal/pkg/util/crypt"
	"github.com/apptainer/apptainer/internal/pkg/util/priv"
	"github.com/apptainer/apptainer/internal/pkg/util/starter"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
)

// CleanupContainer is called from master after the MonitorContainer returns.
// It is responsible for ensuring that the container has been properly torn down.
//
// Additional privileges may be gained when running
// in suid flow. However, when a user namespace is requested and it is not
// a hybrid workflow (e.g. fakeroot), then there is no privileged saved uid
// and thus no additional privileges can be gained.
//
// For better understanding of runtime flow in general refer to
// https://github.com/opencontainers/runtime-spec/blob/master/runtime.md#lifecycle.
// CleanupContainer is performing step 8/9 here.
func (e *EngineOperations) CleanupContainer(ctx context.Context, fatal error, status syscall.WaitStatus) error {
	sylog.Debugf("Cleanup container")

	// close the connection between apptainer and apptheus
	if e.CommonConfig.ApptheusSocket != nil {
		if err := e.CommonConfig.ApptheusSocket.Close(); err != nil {
			sylog.Warningf("failed to close the aptainer connection with apptheus: %v", err)
		}
	}
	// firstly stop all fuse drivers before any image removal
	// by image driver interruption or image cleanup for hybrid
	// fakeroot workflow
	e.stopFuseDrivers()

	if imageDriver != nil {
		if err := umount(); err != nil {
			sylog.Infof("Cleanup error: %s", err)
		}
	}

	if tempDir := e.EngineConfig.GetDeleteTempDir(); tempDir != "" {
		sylog.Verbosef("Removing image tempDir %s", tempDir)
		sylog.Infof("Cleaning up image...")

		var err error

		if e.EngineConfig.GetFakeroot() && os.Getuid() != 0 {
			// this is required when we are using SUID workflow
			// because master process is not in the fakeroot
			// context and can get permission denied error during
			// image removal, so we execute "rm -rf /tmp/image" via
			// the fakeroot engine
			err = fakerootCleanup(tempDir)
		} else {
			err = types.FixPerms(tempDir)
			if err != nil {
				sylog.Debugf("FixPerms had a problem: %v", err)
			}
			err = os.RemoveAll(tempDir)
		}
		if err != nil {
			sylog.Errorf("failed to delete container image tempDir %s: %s", tempDir, err)
		}
	}

	if networkSetup != nil {
		var dropPrivilege priv.DropPrivFunc

		net := e.EngineConfig.GetNetwork()

		// If a CNI configuration was allowed as non-root (or fakeroot)
		if net != "none" && os.Geteuid() != 0 {
			dropPrivilege, _ = priv.Escalate()
		}
		sylog.Debugf("Cleaning up CNI network config %s", net)
		if err := networkSetup.DelNetworks(ctx); err != nil {
			sylog.Errorf("could not delete networks: %v", err)
		}
		if dropPrivilege != nil {
			dropPrivilege()
		}
	}

	if cgroupsManager != nil {
		if err := cgroupsManager.Destroy(); err != nil {
			sylog.Warningf("failed to remove cgroup configuration: %v", err)
		}
	}

	if cryptDev != "" && imageDriver == nil {
		if err := cleanupCrypt(cryptDev); err != nil {
			sylog.Errorf("could not cleanup crypt: %v", err)
		}
	}

	if e.EngineConfig.GetInstance() {
		file, err := instance.Get(e.CommonConfig.ContainerID, instance.AppSubDir)
		if err != nil {
			return err
		}
		return file.Delete()
	}

	return nil
}

func umount() (err error) {
	var errs []string
	var oldEffective uint64

	caps := uint64(0)
	caps |= uint64(1 << capabilities.Map["CAP_SYS_ADMIN"].Value)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oldEffective, err = capabilities.SetProcessEffective(caps)
	if err != nil {
		return fmt.Errorf("error setting CAP_SYS_ADMIN: %v", err)
	}
	defer func() {
		_, e := capabilities.SetProcessEffective(oldEffective)
		if e != nil && len(errs) == 0 {
			errs = []string{"error restoring capabilities: " + e.Error()}
		}
	}()

	// empty target to signify to driver we are entering in the stop phase
	imageDriver.Stop("")

	// gocryptfs related temp folders
	var gocryptfsTmp []string
	for i := len(umountPoints) - 1; i >= 0; i-- {
		p := umountPoints[i]
		sylog.Debugf("Umount %s", p)
		if strings.Contains(p, "gocryptfs-") {
			gocryptfsTmp = append(gocryptfsTmp, p)
		}
		retries := 0
	retry:
		err = syscall.Unmount(p, 0)
		// ignore EINVAL meaning it's not a mount point
		if err != nil && err.(syscall.Errno) != syscall.EINVAL {
			// when rootfs mount point is a sandbox, the unmount
			// fail more often with EBUSY, but it's just a matter of
			// time before resources are released by the kernel so we
			// retry until the unmount operation succeed (retries 10 times)
			if err.(syscall.Errno) == syscall.EBUSY {
				if retries < 10 {
					retries++
					goto retry
				}
				// otherwise give up and do a lazy unmount
				sylog.Debugf("Busy, doing lazy umount %s", p)
				err = syscall.Unmount(p, syscall.MNT_DETACH)
			}
			if err != nil {
				errs = append(errs, fmt.Sprintf("while unmounting %s directory: %s", p, err))
			}
		}
		if imageDriver == nil {
			continue
		}
		err = imageDriver.Stop(p)
		if err != nil {
			errs = append(errs, fmt.Sprintf("while stopping driver for %s: %s", p, err))
		}
	}

	if len(gocryptfsTmp) > 0 {
		dir := filepath.Dir(gocryptfsTmp[0])
		defer os.RemoveAll(dir)
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, ", "))
}

func cleanupCrypt(path string) error {
	if err := umount(); err != nil {
		return err
	}

	devName := filepath.Base(path)

	cryptDev := &crypt.Device{}
	if err := cryptDev.CloseCryptDevice(devName); err != nil {
		return fmt.Errorf("unable to delete crypt device: %s", devName)
	}

	return nil
}

func fakerootCleanup(path string) error {
	rm, err := bin.FindBin("rm")
	if err != nil {
		return err
	}

	command := []string{rm, "-rf", path}

	sylog.Debugf("Calling fakeroot engine to execute %q", strings.Join(command, " "))

	cfg := &config.Common{
		EngineName:   fakerootConfig.Name,
		ContainerID:  "fakeroot",
		EngineConfig: &fakerootConfig.EngineConfig{Args: command},
	}

	return starter.Run(
		"Apptainer fakeroot",
		cfg,
		starter.UseSuid(true),
	)
}
