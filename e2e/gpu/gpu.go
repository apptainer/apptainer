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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
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
			args:        []string{"--contain", "--intel-hpu", logsEnv, imagePath, "hl-smi"},
			expectMatch: e2e.ExpectOutput(e2e.ContainMatch, "no AIPs available"),
			expectExit:  1,
		},
		{
			name:       "UserContainAllDevices",
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

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"nvidia":       c.testNvidiaLegacy,
		"nvccli":       c.testNvCCLI,
		"rocm":         c.testRocm,
		"intel-hpu":    c.testIntelHpu,
		"build nvidia": c.testBuildNvidiaLegacy,
		"build nvccli": c.testBuildNvCCLI,
		"build rocm":   c.testBuildRocm,
	}
}
