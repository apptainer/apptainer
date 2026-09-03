// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"debug/elf"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	utilgpu "github.com/apptainer/apptainer/internal/pkg/util/gpu"
)

var buildDefinition = `Bootstrap: localimage
From: %[1]s

%%setup
	touch $APPTAINER_ROOTFS%[2]s
	touch $APPTAINER_ROOTFS%[3]s
%%post
	%[4]s
%%test
	%[4]s
`

type ctx struct {
	env e2e.TestEnv
}

func (c ctx) testNvidiaLegacy(t *testing.T) {
	require.Nvidia(t)
	// Use Ubuntu 20.04 as this is a recent distro officially supported by Nvidia CUDA.
	// We can't use our test image as it's alpine based and we need a compatible glibc.
	imageURL := "docker://ubuntu:20.04"
	imageFile, err := fs.MakeTmpFile("", "test-nvidia-legacy-", 0o755)
	if err != nil {
		t.Fatalf("Could not create test file: %v", err)
	}
	imageFile.Close()
	imagePath := imageFile.Name()
	defer os.Remove(imagePath)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs("--force", imagePath, imageURL),
		e2e.ExpectExit(0),
	)

	// Basic test that we can run the bound in `nvidia-smi` which *should* be on the PATH
	tests := []struct {
		name    string
		profile e2e.Profile
		args    []string
		env     []string
	}{
		{
			name:    "User",
			profile: e2e.UserProfile,
			args:    []string{"--nv", imagePath, "nvidia-smi"},
		},
		{
			name:    "UserContain",
			profile: e2e.UserProfile,
			args:    []string{"--contain", "--nv", imagePath, "nvidia-smi"},
		},
		{
			name:    "UserNamespace",
			profile: e2e.UserNamespaceProfile,
			args:    []string{"--nv", imagePath, "nvidia-smi"},
		},
		{
			name:    "Fakeroot",
			profile: e2e.FakerootProfile,
			args:    []string{"--nv", imagePath, "nvidia-smi"},
		},
		{
			name:    "Root",
			profile: e2e.RootProfile,
			args:    []string{"--nv", imagePath, "nvidia-smi"},
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.WithEnv(tt.env),
			e2e.ExpectExit(0),
		)
	}

	// The driver's GBM backend, which the ld cache does not list, is bound
	// with the libraries and named in GBM_BACKENDS_PATH ahead of the
	// container's own backend directory.
	backends, _ := filepath.Glob("/usr/lib*/gbm/nvidia-drm_gbm.so")
	multiarch, _ := filepath.Glob("/usr/lib/*/gbm/nvidia-drm_gbm.so")
	if len(backends)+len(multiarch) > 0 {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest("GBMBackend"),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs("--nv", imagePath, "sh", "-c", "ls /.singularity.d/libs/nvidia-drm_gbm.so && echo $GBM_BACKENDS_PATH"),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.RegexMatch, `(?m)^/\.singularity\.d/libs(:|$)`)),
		)
	}
}

func (c ctx) testNvidiaCompat32(t *testing.T) {
	require.Nvidia(t)
	require.NvidiaCompat32(t)
	// Use a Debian-based image, as we can't use our test image which is
	// alpine based and we need a compatible glibc.
	e2e.EnsureDebianImage(t, c.env)
	imagePath := c.env.DebianImagePath

	// The 32-bit libraries are only provisioned on request, and are bound
	// into a directory of their own so that they do not shadow the 64-bit
	// libraries of the same name.
	tests := []struct {
		name        string
		args        []string
		expectMatch e2e.ApptainerCmdResultOp
	}{
		{
			// Without --compat32 the 32-bit library directory is absent.
			name: "NoCompat32",
			args: []string{"--nv", imagePath, "test", "!", "-d", "/.singularity.d/libs32"},
		},
		{
			name:        "Compat32",
			args:        []string{"--nv", "--compat32", imagePath, "sh", "-c", "ls /.singularity.d/libs32"},
			expectMatch: e2e.ExpectOutput(e2e.ContainMatch, ".so"),
		},
		{
			name:        "Compat32LdLibraryPath",
			args:        []string{"--nv", "--compat32", imagePath, "sh", "-c", "echo $LD_LIBRARY_PATH"},
			expectMatch: e2e.ExpectOutput(e2e.ContainMatch, "/.singularity.d/libs32"),
		},
		{
			// The 64-bit libraries must still be provisioned as usual.
			name: "Compat32KeepsNv",
			args: []string{"--nv", "--compat32", imagePath, "nvidia-smi"},
		},
		{
			name:        "Compat32RequiresGPUFlag",
			args:        []string{"--compat32", imagePath, "true"},
			expectMatch: e2e.ExpectError(e2e.ContainMatch, "Ignoring --compat32"),
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0, tt.expectMatch),
		)
	}
}

func (c ctx) testNvCCLI(t *testing.T) {
	require.Nvidia(t)
	require.NvCCLI(t)
	// Use Ubuntu 20.04 as this is a recent distro officially supported by Nvidia CUDA.
	// We can't use our test image as it's alpine based and we need a compatible glibc.
	imageURL := "docker://ubuntu:20.04"
	imageFile, err := fs.MakeTmpFile("", "test-nvccli-", 0o755)
	if err != nil {
		t.Fatalf("Could not create test file: %v", err)
	}
	imageFile.Close()
	imagePath := imageFile.Name()
	defer os.Remove(imagePath)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs("--force", imagePath, imageURL),
		e2e.ExpectExit(0),
	)

	// Basic test that we can run the bound in `nvidia-smi` which *should* be on the PATH
	tests := []struct {
		name        string
		profile     e2e.Profile
		args        []string
		env         []string
		expectExit  int
		expectMatch e2e.ApptainerCmdResultOp
	}{
		{
			name:       "User",
			profile:    e2e.RootProfile,
			args:       []string{"--nvccli", imagePath, "nvidia-smi"},
			expectExit: 0,
		},
		{
			// With --contain, we should only see NVIDIA_VISIBLE_DEVICES configured GPUs
			name:        "UserContainNoDevices",
			profile:     e2e.RootProfile,
			args:        []string{"--contain", "--nvccli", imagePath, "nvidia-smi"},
			expectMatch: e2e.ExpectOutput(e2e.ContainMatch, "No devices were found"),
			expectExit:  6,
		},
		{
			name:       "UserContainAllDevices",
			profile:    e2e.RootProfile,
			args:       []string{"--contain", "--nvccli", imagePath, "nvidia-smi"},
			env:        []string{"NVIDIA_VISIBLE_DEVICES=all"},
			expectExit: 0,
		},
		{
			// If we only request compute, not utility, then nvidia-smi should not be present
			name:        "UserNoUtility",
			profile:     e2e.RootProfile,
			args:        []string{"--nvccli", imagePath, "nvidia-smi"},
			env:         []string{"NVIDIA_DRIVER_CAPABILITIES=compute"},
			expectMatch: e2e.ExpectError(e2e.ContainMatch, "\"nvidia-smi\": executable file not found in $PATH"),
			expectExit:  255,
		},
		{
			// Test "graphics" capability without privileges (issue #2033)
			name:       "UserGraphics",
			profile:    e2e.UserProfile,
			args:       []string{"--nvccli", imagePath, "true"},
			env:        []string{"NVIDIA_DRIVER_CAPABILITIES=graphics"},
			expectExit: 0,
		},
		{
			// Require CUDA version >8 should be fine!
			name:       "UserValidRequire",
			profile:    e2e.RootProfile,
			args:       []string{"--nvccli", imagePath, "nvidia-smi"},
			env:        []string{"NVIDIA_REQUIRE_CUDA=cuda>8"},
			expectExit: 0,
		},
		{
			// Require CUDA version >999 should not be satisfied
			name:        "UserInvalidRequire",
			profile:     e2e.RootProfile,
			args:        []string{"--nvccli", imagePath, "nvidia-smi"},
			env:         []string{"NVIDIA_REQUIRE_CUDA=cuda>999"},
			expectMatch: e2e.ExpectError(e2e.ContainMatch, "requirement error: unsatisfied condition: cuda>99"),
			expectExit:  255,
		},
		{
			name:    "UserNamespace",
			profile: e2e.UserNamespaceProfile,
			args:    []string{"--nvccli", imagePath, "nvidia-smi"},
		},
		{
			name:    "UserNamespaceWritable",
			profile: e2e.UserNamespaceProfile,
			args:    []string{"--nvccli", "--writable", imagePath, "nvidia-smi"},
		},
		{
			name:    "Fakeroot",
			profile: e2e.FakerootProfile,
			args:    []string{"--nvccli", imagePath, "nvidia-smi"},
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.WithEnv(tt.env),
			e2e.ExpectExit(tt.expectExit, tt.expectMatch),
		)
	}
}

func (c ctx) testRocm(t *testing.T) {
	require.Rocm(t)
	require.Command(t, "lsmod")

	// rocminfo now needs lsmod - do a brittle bind in for simplicity.
	lsmod, err := exec.LookPath("lsmod")
	if err != nil {
		t.Fatalf("while finding lsmod: %v", err)
	}

	// Use Ubuntu 22.04 as this is the most recent distro officially supported by ROCm.
	// We can't use our test image as it's alpine based and we need a compatible glibc.
	imageURL := "docker://ubuntu:22.04"
	imageFile, err := fs.MakeTmpFile("", "test-rocm-", 0o755)
	if err != nil {
		t.Fatalf("Could not create test file: %v", err)
	}
	imageFile.Close()
	imagePath := imageFile.Name()
	defer os.Remove(imagePath)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs("--force", imagePath, imageURL),
		e2e.ExpectExit(0),
	)

	// Basic test that we can run the bound in `rocminfo` which *should* be on the PATH
	tests := []struct {
		name    string
		profile e2e.Profile
		args    []string
	}{
		{
			name:    "User",
			profile: e2e.UserProfile,
			args:    []string{"-B", lsmod, "--rocm", imagePath, "rocminfo"},
		},
		{
			name:    "UserContain",
			profile: e2e.UserProfile,
			args:    []string{"-B", lsmod, "--contain", "--rocm", imagePath, "rocminfo"},
		},
		{
			name:    "UserNamespace",
			profile: e2e.UserNamespaceProfile,
			args:    []string{"-B", lsmod, "--rocm", imagePath, "rocminfo"},
		},
		{
			name:    "Fakeroot",
			profile: e2e.FakerootProfile,
			args:    []string{"-B", lsmod, "--rocm", imagePath, "rocminfo"},
		},
		{
			name:    "Root",
			profile: e2e.RootProfile,
			args:    []string{"-B", lsmod, "--rocm", imagePath, "rocminfo"},
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0),
		)
	}
}

//nolint:dupl
func (c ctx) testIntelHpu(t *testing.T) {
	require.IntelHpu(t)

	imageURL := "docker://vault.habana.ai/gaudi-docker/1.20.0/ubuntu22.04/habanalabs/pytorch-installer-2.6.0:latest"
	imageFile, err := fs.MakeTmpFile("", "test-hpu-", 0o755)
	if err != nil {
		t.Fatalf("Could not create test file: %v", err)
	}

	imageFile.Close()
	imagePath := imageFile.Name()
	defer os.Remove(imagePath)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs("--force", imagePath, imageURL),
		e2e.ExpectExit(0),
	)

	// Need to override default logs location
	// which is /var/log/habana_logs since it's read-only
	// without --writable-tmpfs flag
	logsEnv := "--env=HABANA_LOGS=/tmp/habana_logs"

	// Basic test that we can see HPU devices and select devices
	tests := []struct {
		name        string
		profile     e2e.Profile
		args        []string
		expectExit  int
		expectMatch e2e.ApptainerCmdResultOp
	}{
		{
			name:        "UserContainNoDevices",
			profile:     e2e.UserProfile,
			args:        []string{"--contain", "--intel-hpu", logsEnv, "--env=HABANA_VISIBLE_DEVICES=''", imagePath, "hl-smi"},
			expectMatch: e2e.ExpectOutput(e2e.ContainMatch, "no AIPs available"),
			expectExit:  1,
		},
		{
			name:       "UserContainAllDevicesImplicit",
			profile:    e2e.UserProfile,
			args:       []string{"--contain", "--intel-hpu", logsEnv, imagePath, "hl-smi"},
			expectExit: 0,
		},
		{
			name:       "UserContainAllDevicesExplicit",
			profile:    e2e.UserProfile,
			args:       []string{"--contain", "--intel-hpu", logsEnv, "--env=HABANA_VISIBLE_DEVICES=all", imagePath, "hl-smi"},
			expectExit: 0,
		},
		{
			name:        "UserContainSelectedDevice",
			profile:     e2e.UserProfile,
			args:        []string{"--contain", "--intel-hpu", logsEnv, "--env=HABANA_VISIBLE_DEVICES=0", imagePath, "hl-smi", "--query-aip=index", "--format=csv,noheader"},
			expectMatch: e2e.ExpectOutput(e2e.ExactMatch, "0"),
			expectExit:  0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit, tt.expectMatch),
		)
	}
}

//nolint:dupl
func (c ctx) testBuildNvidiaLegacy(t *testing.T) {
	require.Nvidia(t)

	// ignore the error as it's already done in the require call above
	nvsmi, _ := exec.LookPath("nvidia-smi")

	// Use Ubuntu 20.04 as this is the most recent distro officially supported by Nvidia CUDA.
	// We can't use our test image as it's alpine based and we need a compatible glibc.
	imageURL := "docker://ubuntu:20.04"

	tmpdir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "build-nvidia-legacy", "build with nvidia")
	defer cleanup(t)

	sourceImage := filepath.Join(tmpdir, "source")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sourceImage, imageURL),
		e2e.ExpectExit(0),
	)

	// Basic test that we can run the bound in `rocminfo` which *should* be on the PATH
	tests := []struct {
		name      string
		profile   e2e.Profile
		setNvFlag bool
		exit      int
	}{
		{
			name:      "WithNvRoot",
			profile:   e2e.RootProfile,
			setNvFlag: true,
			exit:      0,
		},
		{
			name:      "WithNvFakeroot",
			profile:   e2e.FakerootProfile,
			setNvFlag: true,
			exit:      0,
		},
		{
			name:      "WithoutNvRoot",
			profile:   e2e.RootProfile,
			setNvFlag: false,
			exit:      255,
		},
		{
			name:      "WithoutNvFakeroot",
			profile:   e2e.FakerootProfile,
			setNvFlag: false,
			exit:      255,
		},
	}

	rawDef := fmt.Sprintf(buildDefinition, sourceImage, nvsmi, "", "nvidia-smi")

	for _, tt := range tests {
		defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(rawDef))
		sandboxImage := filepath.Join(tmpdir, "sandbox-"+tt.name)

		args := []string{}
		if tt.setNvFlag {
			args = append(args, "--nv")
		}
		args = append(args, "-F", "--sandbox", sandboxImage, defFile)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.ExpectExit(tt.exit),
			e2e.PostRun(func(t *testing.T) {
				if t.Failed() {
					return
				}
				defer os.RemoveAll(sandboxImage)
			}),
		)
	}
}

func (c ctx) testBuildNvCCLI(t *testing.T) {
	require.Nvidia(t)
	require.NvCCLI(t)

	// ignore the error as it's already done in the require call above
	nvsmi, _ := exec.LookPath("nvidia-smi")

	// Use Ubuntu 20.04 as this is the most recent distro officially supported by Nvidia CUDA.
	// We can't use our test image as it's alpine based and we need a compatible glibc.
	imageURL := "docker://ubuntu:20.04"

	tmpdir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "build-nvccli", "build with nvccli")
	defer cleanup(t)

	sourceImage := filepath.Join(tmpdir, "source")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sourceImage, imageURL),
		e2e.ExpectExit(0),
	)

	// Basic test that we can run the bound in `rocminfo` which *should* be on the PATH
	tests := []struct {
		name      string
		profile   e2e.Profile
		setNvFlag bool
		exit      int
	}{
		{
			name:      "WithNvccliRoot",
			profile:   e2e.RootProfile,
			setNvFlag: true,
			exit:      0,
		},
		{
			name:      "WithoutNvccliRoot",
			profile:   e2e.RootProfile,
			setNvFlag: false,
			exit:      255,
		},
	}

	rawDef := fmt.Sprintf(buildDefinition, sourceImage, nvsmi, "", "nvidia-smi")

	for _, tt := range tests {
		defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(rawDef))
		sandboxImage := filepath.Join(tmpdir, "sandbox-"+tt.name)

		args := []string{}
		if tt.setNvFlag {
			args = append(args, "--nvccli")
		}
		args = append(args, "-F", "--sandbox", sandboxImage, defFile)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.ExpectExit(tt.exit),
			e2e.PostRun(func(t *testing.T) {
				if t.Failed() {
					return
				}
				os.RemoveAll(sandboxImage)
			}),
		)
	}
}

//nolint:dupl
func (c ctx) testBuildRocm(t *testing.T) {
	require.Rocm(t)
	require.Command(t, "lsmod")

	// rocminfo now needs lsmod - do a brittle bind in for simplicity.
	lsmod, err := exec.LookPath("lsmod")
	if err != nil {
		t.Fatalf("while finding lsmod: %v", err)
	}

	// ignore the error as it's already done in the require call above
	rocmInfo, _ := exec.LookPath("rocminfo")

	// Use Ubuntu 22.04 as this is the most recent distro officially supported by ROCm.
	// We can't use our test image as it's alpine based and we need a compatible glibc.
	imageURL := "docker://ubuntu:22.04"

	tmpdir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "build-rocm-image", "build with rocm")
	defer cleanup(t)

	sourceImage := filepath.Join(tmpdir, "source")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sourceImage, imageURL),
		e2e.ExpectExit(0),
	)

	// Basic test that we can run the bound in `rocminfo` which *should* be on the PATH
	tests := []struct {
		name        string
		profile     e2e.Profile
		setRocmFlag bool
		exit        int
	}{
		{
			name:        "WithRocmRoot",
			profile:     e2e.RootProfile,
			setRocmFlag: true,
			exit:        0,
		},
		{
			name:        "WithRocmFakeroot",
			profile:     e2e.FakerootProfile,
			setRocmFlag: true,
			exit:        0,
		},
		{
			name:        "WithoutRocmRoot",
			profile:     e2e.RootProfile,
			setRocmFlag: false,
			exit:        255,
		},
		{
			name:        "WithoutRocmFakeroot",
			profile:     e2e.FakerootProfile,
			setRocmFlag: false,
			exit:        255,
		},
	}

	rawDef := fmt.Sprintf(buildDefinition, sourceImage, rocmInfo, lsmod, "rocminfo")

	for _, tt := range tests {
		defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(rawDef))
		sandboxImage := filepath.Join(tmpdir, "sandbox-"+tt.name)

		args := []string{}
		if tt.setRocmFlag {
			args = append(args, "--rocm")
		}
		args = append(args, "-B", lsmod, "--force", "--sandbox", sandboxImage, defFile)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.ExpectExit(tt.exit),
			e2e.PostRun(func(t *testing.T) {
				if t.Failed() {
					return
				}
				defer os.RemoveAll(sandboxImage)
			}),
		)
	}
}

// gpuStandInLib is the name given to the stand-in library used by
// testGpuLibraryPath. It matches the libEGL_installertest.so entry of
// nvliblist.conf, which is listed there but is not part of a driver
// installation, so a library of this name reaching the container can only
// have come from the configured 'gpu library path', whether or not the host
// has a GPU driver installed.
const gpuStandInLib = "libEGL_installertest.so.1"

// gpuDanglingLib is the name given to the dangling symlink used by
// testGpuLibraryPath. It matches the libcuda.so entry of nvliblist.conf, so
// it is looked for, and it does not resolve to a library, so it must never be
// provisioned.
const gpuDanglingLib = "libcuda.so.1"

// hostLibrary returns the path of a shared library on the host, built for the
// same machine and ELF class as the libraries that --nv provisions, to stand
// in for a GPU driver library.
func hostLibrary(t *testing.T) string {
	t.Helper()

	self, err := elf.Open("/proc/self/exe")
	if err != nil {
		t.Skipf("Could not read the ELF header of the test binary: %v", err)
	}
	machine, class := self.Machine, self.Class
	self.Close()

	// The C library is present on any host able to run these tests. Cover
	// where the multiarch, lib64 and musl layouts keep it.
	for _, glob := range []string{
		"/lib/*/libc.so.*",
		"/lib64/libc.so.*",
		"/lib/libc.so.*",
		"/usr/lib/*/libc.so.*",
		"/usr/lib64/libc.so.*",
		"/usr/lib/libc.so.*",
		"/lib/libc.musl-*.so.*",
		"/usr/lib/libc.musl-*.so.*",
	} {
		matches, err := filepath.Glob(glob)
		if err != nil {
			t.Fatalf("Could not search for a host library: %v", err)
		}
		for _, match := range matches {
			lib, err := elf.Open(match)
			if err != nil {
				continue
			}
			matching := lib.Machine == machine && lib.Class == class
			lib.Close()
			if matching {
				return match
			}
		}
	}

	t.Skip("Could not find a host library to stand in for a GPU driver library")
	return ""
}

// noLdCacheEnv returns an environment in which apptainer has no ld cache to
// read, by creating an ldconfig which fails to run under dir and putting it at
// the front of PATH, where apptainer finds it in place of the system one. It
// stands in for a host which does not ship ldconfig, or which does not
// populate an ld.so cache. "ldconfig.real" is created as well, as apptainer
// looks for that name first.
func noLdCacheEnv(t *testing.T, dir string) []string {
	t.Helper()

	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("Could not create directory: %v", err)
	}
	script := []byte("#!/bin/sh\necho 'ldconfig: no ld.so cache on this host' >&2\nexit 1\n")
	for _, name := range []string{"ldconfig", "ldconfig.real"} {
		if err := os.WriteFile(filepath.Join(binDir, name), script, 0o755); err != nil {
			t.Fatalf("Could not create %s: %v", name, err)
		}
	}

	return append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
}

// testGpuLibraryPath checks the 'gpu library path' directive of
// apptainer.conf, which names the directories to search for the GPU driver
// libraries listed in nvliblist.conf and rocmliblist.conf, ahead of and
// without needing the ld cache.
//
// A GPU is not required: a copy of a host library, named after an entry of
// nvliblist.conf that no driver provides, stands in for a driver library, so
// whether --nv provisions it tells us whether the configured directory was
// searched.
func (c ctx) testGpuLibraryPath(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "gpu-library-path-", "gpu library path")
	defer cleanup(t)

	// libDir holds the stand-in library. firstDir holds nothing usable, and
	// is configured ahead of libDir to check that the whole of the
	// configured list is searched, and that an entry which does not resolve
	// is passed over.
	libDir := filepath.Join(tmpDir, "lib")
	firstDir := filepath.Join(tmpDir, "first")
	for _, dir := range []string{libDir, firstDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Could not create directory: %v", err)
		}
	}
	if err := fs.CopyFile(hostLibrary(t), filepath.Join(libDir, gpuStandInLib), 0o755); err != nil {
		t.Fatalf("Could not create stand-in library: %v", err)
	}
	if err := os.Symlink(filepath.Join(firstDir, "removed"), filepath.Join(firstDir, gpuDanglingLib)); err != nil {
		t.Fatalf("Could not create symlink: %v", err)
	}

	noLdCache := noLdCacheEnv(t, tmpDir)
	standInLib := "/.singularity.d/libs/" + gpuStandInLib

	tests := []struct {
		name        string
		env         []string
		libraryPath string
		args        []string
		exit        int
		expectMatch e2e.ApptainerCmdResultOp
	}{
		{
			// Nothing but a configured directory can provide the stand-in
			// library, so it must not turn up when none is configured.
			name: "Unconfigured",
			args: []string{"--nv", c.env.ImagePath, "test", "-e", standInLib},
			exit: 1,
		},
		{
			// The configured directory is searched alongside the ld cache.
			name:        "Configured",
			libraryPath: libDir,
			args:        []string{"--nv", c.env.ImagePath, "test", "-e", standInLib},
			exit:        0,
		},
		{
			// With neither an ld cache nor a configured directory there is
			// nowhere left to look, and the warning has to say so.
			name:        "NoLdCacheUnconfigured",
			env:         noLdCache,
			args:        []string{"--nv", c.env.ImagePath, "test", "-e", standInLib},
			exit:        1,
			expectMatch: e2e.ExpectError(e2e.ContainMatch, "no 'gpu library path' is configured in apptainer.conf"),
		},
		{
			// The configured directory is enough on its own: the ld cache is
			// optional, and doing without it is not worth a warning.
			name:        "NoLdCacheConfigured",
			env:         noLdCache,
			libraryPath: libDir,
			args:        []string{"--nv", c.env.ImagePath, "test", "-e", standInLib},
			exit:        0,
			expectMatch: e2e.ExpectError(e2e.UnwantedContainMatch, "Could not read the ld cache"),
		},
		{
			// Every configured directory is searched, not only the first.
			name:        "NoLdCacheSecondDirectory",
			env:         noLdCache,
			libraryPath: firstDir + "," + libDir,
			args:        []string{"--nv", c.env.ImagePath, "test", "-e", standInLib},
			exit:        0,
		},
		{
			// The dangling symlink is the only candidate for its name here,
			// and it must be passed over rather than provisioned.
			name:        "NoLdCacheDangling",
			env:         noLdCache,
			libraryPath: firstDir + "," + libDir,
			args:        []string{"--nv", c.env.ImagePath, "test", "!", "-e", "/.singularity.d/libs/" + gpuDanglingLib},
			exit:        0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.PreRun(func(t *testing.T) {
				if tt.libraryPath != "" {
					e2e.SetDirective(t, c.env, "gpu library path", tt.libraryPath)
				}
			}),
			e2e.PostRun(func(t *testing.T) {
				if tt.libraryPath != "" {
					e2e.ResetDirective(t, c.env, "gpu library path")
				}
			}),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.WithEnv(tt.env),
			e2e.ExpectExit(tt.exit, tt.expectMatch),
		)
	}
}

// testNvidiaLibraryPath checks that the NVIDIA driver libraries of a host with
// a GPU are provisioned from the directories named by 'gpu library path'
// alone, on a host where apptainer has no ld cache to fall back on.
func (c ctx) testNvidiaLibraryPath(t *testing.T) {
	require.Nvidia(t)
	// Use a Debian-based image, as we can't use our test image which is
	// alpine based and we need a compatible glibc.
	e2e.EnsureDebianImage(t, c.env)
	imagePath := c.env.DebianImagePath

	// Find where this host keeps its driver libraries by resolving them the
	// usual way, through the ld cache. Configuring those directories has to
	// give --nv the same libraries with the ld cache out of reach.
	libs, _, _, err := utilgpu.NvidiaPaths(buildcfg.NVIDIALIBS_FILE)
	if err != nil {
		t.Skipf("Could not resolve the NVIDIA libraries of this host: %v", err)
	}
	var libraryPath []string
	for _, lib := range libs {
		if dir := filepath.Dir(lib); !slices.Contains(libraryPath, dir) {
			libraryPath = append(libraryPath, dir)
		}
	}
	if len(libraryPath) == 0 {
		t.Skip("No NVIDIA libraries found on this host")
	}
	t.Logf("Setting 'gpu library path' to %s", strings.Join(libraryPath, ", "))

	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "nvidia-library-path-", "nvidia gpu library path")
	defer cleanup(t)

	noLdCache := noLdCacheEnv(t, tmpDir)

	tests := []struct {
		name        string
		env         []string
		libraryPath string
		args        []string
		expectMatch e2e.ApptainerCmdResultOp
	}{
		{
			// Searching the configured directories ahead of the ld cache
			// must not disturb provisioning when both are available.
			name:        "Configured",
			libraryPath: strings.Join(libraryPath, ","),
			args:        []string{"--nv", imagePath, "nvidia-smi"},
		},
		{
			// Without an ld cache and without the directive there is nothing
			// left to provision the driver libraries from.
			name:        "NoLdCacheUnconfigured",
			env:         noLdCache,
			args:        []string{"--nv", imagePath, "sh", "-c", "ls /.singularity.d/libs | wc -l"},
			expectMatch: e2e.ExpectOutput(e2e.ExactMatch, "0"),
		},
		{
			// The configured directories are enough on their own to run a
			// program against the GPU.
			name:        "NoLdCacheConfigured",
			env:         noLdCache,
			libraryPath: strings.Join(libraryPath, ","),
			args:        []string{"--nv", imagePath, "nvidia-smi"},
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.PreRun(func(t *testing.T) {
				if tt.libraryPath != "" {
					e2e.SetDirective(t, c.env, "gpu library path", tt.libraryPath)
				}
			}),
			e2e.PostRun(func(t *testing.T) {
				if tt.libraryPath != "" {
					e2e.ResetDirective(t, c.env, "gpu library path")
				}
			}),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.WithEnv(tt.env),
			e2e.ExpectExit(0, tt.expectMatch),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	// The 'gpu library path' tests change apptainer.conf, so they must not
	// run alongside anything else.
	np := testhelper.NoParallel

	return testhelper.Tests{
		"nvidia":                  c.testNvidiaLegacy,
		"nvidia compat32":         c.testNvidiaCompat32,
		"nvccli":                  c.testNvCCLI,
		"rocm":                    c.testRocm,
		"intel-hpu":               c.testIntelHpu,
		"build nvidia":            c.testBuildNvidiaLegacy,
		"build nvccli":            c.testBuildNvCCLI,
		"build rocm":              c.testBuildRocm,
		"gpu library path":        np(c.testGpuLibraryPath),
		"nvidia gpu library path": np(c.testNvidiaLibraryPath),
	}
}
