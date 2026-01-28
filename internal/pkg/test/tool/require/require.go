// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package require

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/security/apparmor"
	"github.com/apptainer/apptainer/internal/pkg/security/seccomp"
	"github.com/apptainer/apptainer/internal/pkg/security/selinux"
	"github.com/apptainer/apptainer/internal/pkg/util/rpm"
	"github.com/apptainer/apptainer/pkg/network"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	"github.com/apptainer/apptainer/pkg/util/slice"
	"github.com/google/uuid"
	"github.com/opencontainers/cgroups"
)

var (
	hasUserNamespace     bool
	hasUserNamespaceOnce sync.Once
)

// UserNamespace checks that the current test could use
// user namespace, if user namespaces are not enabled or
// supported, the current test is skipped with a message.
func UserNamespace(t *testing.T) {
	// not performance critical, just save extra execution
	// to get the same result
	hasUserNamespaceOnce.Do(func() {
		// user namespace is a bit special, as there is no simple
		// way to detect if it's supported or enabled via a call
		// on /proc/self/ns/user, the easiest and reliable way seems
		// to directly execute a command by requesting user namespace
		cmd := exec.Command("/bin/true")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
		// no error means user namespaces are enabled
		err := cmd.Run()
		hasUserNamespace = err == nil
		if !hasUserNamespace {
			t.Logf("Could not use user namespaces: %s", err)
		}
	})
	if !hasUserNamespace {
		t.Skipf("user namespaces seems not enabled or supported")
	}
}

var (
	supportNetwork     bool
	supportNetworkOnce sync.Once
)

// Network check that bridge network is supported by
// system, if not the current test is skipped with a
// message.
func Network(t *testing.T) {
	supportNetworkOnce.Do(func() {
		logFn := func(err error) {
			t.Logf("Could not use network: %s", err)
		}

		ctx := t.Context()

		cmd := exec.Command("/bin/cat")
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWNET

		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			logFn(err)
			return
		}

		err = cmd.Start()
		if err != nil {
			logFn(err)
			return
		}

		nsPath := fmt.Sprintf("/proc/%d/ns/net", cmd.Process.Pid)

		cniPath := new(network.CNIPath)
		cniPath.Conf = filepath.Join(buildcfg.SYSCONFDIR, "apptainer", "network")
		cniPath.Plugin = filepath.Join(buildcfg.LIBEXECDIR, "apptainer", "cni")
		containerID := "apptainer-e2e-" + uuid.New().String()

		setup, err := network.NewSetup([]string{"bridge"}, containerID, nsPath, cniPath)
		if err != nil {
			logFn(err)
			return
		}
		if err := setup.AddNetworks(ctx); err != nil {
			logFn(err)
			return
		}
		if err := setup.DelNetworks(ctx); err != nil {
			logFn(err)
			return
		}

		stdinPipe.Close()

		if err := cmd.Wait(); err != nil {
			logFn(err)
			return
		}

		supportNetwork = true
	})
	if !supportNetwork {
		t.Skipf("Network (bridge) not supported")
	}
}

// Cgroups checks that any cgroups version is enabled, if not the
// current test is skipped with a message.
func Cgroups(t *testing.T) {
	subsystems, err := cgroups.GetAllSubsystems()
	if err != nil || len(subsystems) == 0 {
		t.Skipf("cgroups not available")
	}
}

// CgroupsV1 checks that legacy cgroups is enabled, if not the
// current test is skipped with a message.
func CgroupsV1(t *testing.T) {
	Cgroups(t)
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skipf("cgroups v1 legacy mode not available")
	}
}

// CgroupsV2 checks that cgroups v2 unified mode is enabled, if not the
// current test is skipped with a message.
func CgroupsV2Unified(t *testing.T) {
	if !cgroups.IsCgroup2UnifiedMode() {
		t.Skipf("cgroups v2 unified mode not available")
	}
}

// CgroupsFreezer checks that cgroup freezer subsystem is
// available, if not the current test is skipped with a
// message
func CgroupsFreezer(t *testing.T) {
	subsystems, err := cgroups.GetAllSubsystems()
	if err != nil {
		t.Skipf("couldn't get cgroups subsystems: %v", err)
	}
	if !slice.ContainsString(subsystems, "freezer") {
		t.Skipf("no cgroups freezer subsystem available")
	}
}

// CgroupsResourceExists checks that the requested controller and resource exist
// in the cgroupfs.
func CgroupsResourceExists(t *testing.T, controller string, resource string) {
	cgs, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		t.Error(err)
	}
	cgPath, ok := cgs[controller]
	if !ok {
		t.Skipf("controller %s cgroup path not found", controller)
	}

	resourcePath := filepath.Join("/sys/fs/cgroup", controller, cgPath, resource)
	if _, err := os.Stat(resourcePath); err != nil {
		t.Skipf("cannot stat resource %s: %s", resource, err)
	}
}

// CroupsV2Delegated checks that the controller is delegated to users.
func CgroupsV2Delegated(t *testing.T, controller string) {
	CgroupsV2Unified(t)
	cgs, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		t.Error(err)
	}

	cgPath, ok := cgs[""]
	if !ok {
		t.Skipf("unified cgroup path not found")
	}

	delegatePath := filepath.Join("/sys/fs/cgroup", cgPath, "cgroup.controllers")

	data, err := os.ReadFile(delegatePath)
	if err != nil {
		t.Skipf("while reading delegation file: %s", err)
	}

	if !strings.Contains(string(data), controller) {
		t.Skipf("%s controller is not delegated", controller)
	}
}

// Nvidia checks that an NVIDIA stack is available
func Nvidia(t *testing.T) {
	nvsmi, err := exec.LookPath("nvidia-smi")
	if err != nil {
		t.Skipf("nvidia-smi not found on PATH: %v", err)
	}
	cmd := exec.Command(nvsmi)
	if err := cmd.Run(); err != nil {
		t.Skipf("nvidia-smi failed to run: %v", err)
	}
}

// NvCCLI checks that nvidia-container-cli is available
func NvCCLI(t *testing.T) {
	_, err := exec.LookPath("nvidia-container-cli")
	if err != nil {
		t.Skipf("nvidia-container-cli not found on PATH: %v", err)
	}
}

// Rocm checks that a Rocm stack is available
func Rocm(t *testing.T) {
	rocminfo, err := exec.LookPath("rocminfo")
	if err != nil {
		t.Skipf("rocminfo not found on PATH: %v", err)
	}
	cmd := exec.Command(rocminfo)
	if output, err := cmd.Output(); err != nil {
		t.Skipf("rocminfo failed to run: %v - %v", err, string(output))
	}
}

// IntelHpu checks that a Gaudi accelerator stack is available
func IntelHpu(t *testing.T) {
	hlsmi, err := exec.LookPath("hl-smi")
	if err != nil {
		t.Skipf("hl-smi not found on PATH: %v", err)
	}
	cmd := exec.Command(hlsmi)
	if output, err := cmd.Output(); err != nil {
		t.Skipf("hl-smi failed to run: %v - %v", err, string(output))
	}
}

// DMTCP checks that a DMTCP stack is available
func DMTCP(t *testing.T) {
	_, err := exec.LookPath("dmtcp_launch")
	if err != nil {
		t.Skipf("dmtcp_launch not found on PATH: %v", err)
	}
}

// Filesystem checks that the current test could use the
// corresponding filesystem, if the filesystem is not
// listed in /proc/filesystems, the current test is skipped
// with a message.
func Filesystem(t *testing.T, fs string) {
	has, err := proc.HasFilesystem(fs)
	if err != nil {
		t.Fatalf("error while checking filesystem presence: %s", err)
	}
	if !has {
		t.Skipf("%s filesystem seems not supported", fs)
	}
}

// Command checks if the provided command is found
// in one the path defined in the PATH environment variable,
// if not found the current test is skipped with a message.
func Command(t *testing.T, command string) {
	_, err := exec.LookPath(command)
	if err != nil {
		t.Skipf("%s command not found in $PATH", command)
	}
}

// Seccomp checks that seccomp is enabled, if not the
// current test is skipped with a message.
func Seccomp(t *testing.T) {
	if !seccomp.Enabled() {
		t.Skipf("seccomp disabled, Apptainer was compiled without the seccomp library")
	}
}

// Apparmor checks that apparmor is enabled. If not, the test is skipped with a
// message.
func Apparmor(t *testing.T) {
	if !apparmor.Enabled() {
		t.Skipf("apparmor is not available")
	}
}

// Selinux checks that selinux is enabled. If not, the test is skipped with a
// message.
func Selinux(t *testing.T) {
	if !selinux.Enabled() {
		t.Skipf("selinux is not available")
	}
}

// Arch checks the test machine has the specified architecture.
// If not, the test is skipped with a message.
func Arch(t *testing.T, arch string) {
	if arch != "" && runtime.GOARCH != arch {
		t.Skipf("test requires architecture %s", arch)
	}
}

// ArchIn checks the test machine is one of the specified archs.
// If not, the test is skipped with a message.
func ArchIn(t *testing.T, archs []string) {
	if len(archs) > 0 {
		b := runtime.GOARCH
		for _, a := range archs {
			if b == a {
				return
			}
		}
		t.Skipf("test requires architecture %s", strings.Join(archs, "|"))
	}
}

// MkfsExt3 checks that mkfs.ext3 is available and
// support -d option to create writable overlay layout.
func MkfsExt3(t *testing.T) {
	mkfs, err := exec.LookPath("mkfs.ext3")
	if err != nil {
		t.Skipf("mkfs.ext3 not found in $PATH")
	}

	buf := new(bytes.Buffer)
	cmd := exec.Command(mkfs, "--help")
	cmd.Stderr = buf
	_ = cmd.Run()

	if !strings.Contains(buf.String(), "[-d ") {
		t.Skipf("mkfs.ext3 is too old and doesn't support -d")
	}
}

func RPMMacro(t *testing.T, name, value string) {
	eval, err := rpm.GetMacro(name)
	if err != nil {
		t.Skipf("Couldn't get value of %s: %s", name, err)
	}

	if eval != value {
		t.Skipf("Need %s as value of %s, got %s", value, name, eval)
	}
}
