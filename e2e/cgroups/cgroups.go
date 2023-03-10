// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"fmt"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/google/uuid"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

//  NOTE
//  ----
//  Tests in this package/topic are run in a mount namespace only. There is
//  no PID namespace, in order that the systemd cgroups manager functionality
//  can be exercised.
//
//  You must take extra care not to leave detached process etc. that will
//  pollute the host PID namespace.
//

// randomName generates a random name instance or OCI container name based on a UUID.
func randomName(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

type ctx struct {
	env e2e.TestEnv
}

// instanceStats tests an instance ability to output stats
func (c *ctx) instanceStats(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	// All tests require root
	tests := []struct {
		name           string
		createArgs     []string
		startErrorCode int
		statsErrorCode int
	}{
		{
			name:           "basic stats create",
			createArgs:     []string{"--memory", "250M", c.env.ImagePath},
			statsErrorCode: 0,
			startErrorCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We always expect stats output, not create
			createExitFunc := []e2e.ApptainerCmdResultOp{}
			instanceName := randomName(t)

			// Start the instance with cgroups for stats
			createArgs := append(tt.createArgs, instanceName)
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("start"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance start"),
				e2e.WithArgs(createArgs...),
				e2e.ExpectExit(tt.startErrorCode, createExitFunc...),
			)

			// Get stats for the instance
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("stats"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance stats"),
				e2e.WithArgs("--no-stream", instanceName),
				e2e.ExpectExit(tt.statsErrorCode,
					// Header (column spacing varies by content)
					e2e.ExpectOutput(e2e.ContainMatch, "INSTANCE NAME"),
					e2e.ExpectOutput(e2e.ContainMatch, "CPU USAGE"),
					e2e.ExpectOutput(e2e.ContainMatch, "MEM USAGE / LIMIT"),
					e2e.ExpectOutput(e2e.ContainMatch, "MEM %"),
					e2e.ExpectOutput(e2e.ContainMatch, "BLOCK I/O"),
					e2e.ExpectOutput(e2e.ContainMatch, "PIDS"),
					e2e.ExpectOutput(e2e.ContainMatch, "PIDS"),
					// Instance name is visible
					e2e.ExpectOutput(e2e.ContainMatch, instanceName),
					// Memory limit is visible
					e2e.ExpectOutput(e2e.ContainMatch, "/ 250MiB"),
				),
			)
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("stop"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance stop"),
				e2e.WithArgs(instanceName),
				e2e.ExpectExit(0),
			)
		})
	}
}

// moved from INSTANCE suite, as testing with systemd cgroup manager requires
// e2e to be run without PID namespace
func (c *ctx) instanceApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name           string
		createArgs     []string
		execArgs       []string
		startErrorCode int
		startErrorOut  string
		execErrorCode  int
		execErrorOut   string
		rootfull       bool
		rootless       bool
	}{
		{
			name:           "nonexistent toml",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/doesnotexist.toml", c.env.ImagePath},
			startErrorCode: 255,
			// e2e test currently only captures the error from the CLI process, not the error displayed by the
			// starter process, so we check for the generic CLI error.
			startErrorOut: "no such file or directory",
			rootfull:      true,
			rootless:      true,
		},
		{
			name:           "invalid toml",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/invalid.toml", c.env.ImagePath},
			startErrorCode: 255,
			// e2e test currently only captures the error from the CLI process, not the error displayed by the
			// starter process, so we check for the generic CLI error.
			startErrorOut: "toml: expected character",
			rootfull:      true,
			rootless:      true,
		},
		{
			name:       "memory limit",
			createArgs: []string{"--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath},
			// We get a CLI 255 error code, not the 137 that the starter receives for an OOM kill
			startErrorCode: 255,
			startErrorOut:  "signal: killed",
			rootfull:       true,
			rootless:       true,
		},
		{
			name:           "cpu success",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/cpu_success.toml", c.env.ImagePath},
			startErrorCode: 0,
			execArgs:       []string{"/bin/true"},
			execErrorCode:  0,
			rootfull:       true,
			rootless:       true,
		},
		{
			name:           "device deny",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath},
			startErrorCode: 0,
			execArgs:       []string{"cat", "/dev/null"},
			execErrorCode:  1,
			execErrorOut:   "Operation not permitted",
			rootfull:       true,
			rootless:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if profile.Privileged() && !tt.rootfull {
				t.Skip()
			}
			if !profile.Privileged() && !tt.rootless {
				t.Skip()
			}

			createExitFunc := []e2e.ApptainerCmdResultOp{}
			if tt.startErrorOut != "" {
				createExitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectError(e2e.ContainMatch, tt.startErrorOut)}
			}
			execExitFunc := []e2e.ApptainerCmdResultOp{}
			if tt.execErrorOut != "" {
				execExitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectError(e2e.ContainMatch, tt.execErrorOut)}
			}
			// pick up a random name
			instanceName := randomName(t)
			joinName := fmt.Sprintf("instance://%s", instanceName)

			createArgs := append(tt.createArgs, instanceName)
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("start"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance start"),
				e2e.WithArgs(createArgs...),
				e2e.ExpectExit(tt.startErrorCode, createExitFunc...),
			)
			if tt.startErrorCode != 0 {
				return
			}

			execArgs := append([]string{joinName}, tt.execArgs...)
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("exec"),
				e2e.WithProfile(profile),
				e2e.WithGlobalOptions("-d"),
				e2e.WithCommand("exec"),
				e2e.WithArgs(execArgs...),
				e2e.WithDir(profile.HostUser(t).Dir),
				e2e.ExpectExit(tt.execErrorCode, execExitFunc...),
			)

			c.env.RunApptainer(
				t,
				e2e.AsSubtest("stop"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance stop"),
				e2e.WithArgs(instanceName),
				e2e.ExpectExit(0),
			)
		})
	}
}

func (c *ctx) instanceApplyRoot(t *testing.T) {
	c.instanceApply(t, e2e.RootProfile)
}

func (c *ctx) instanceApplyRootless(t *testing.T) {
	c.instanceApply(t, e2e.UserProfile)
}

func (c *ctx) instanceStatsRoot(t *testing.T) {
	c.instanceStats(t, e2e.RootProfile)
}

func (c *ctx) instanceStatsRootless(t *testing.T) {
	c.instanceStats(t, e2e.UserProfile)
}

func (c *ctx) actionApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name            string
		args            []string
		expectErrorCode int
		expectErrorOut  string
		rootfull        bool
		rootless        bool
	}{
		{
			name:            "nonexistent toml",
			args:            []string{"--apply-cgroups", "testdata/cgroups/doesnotexist.toml", c.env.ImagePath, "/bin/sleep", "5"},
			expectErrorCode: 255,
			expectErrorOut:  "no such file or directory",
			rootfull:        true,
			rootless:        true,
		},
		{
			name:            "invalid toml",
			args:            []string{"--apply-cgroups", "testdata/cgroups/invalid.toml", c.env.ImagePath, "/bin/sleep", "5"},
			expectErrorCode: 255,
			expectErrorOut:  "toml: expected character",
			rootfull:        true,
			rootless:        true,
		},
		{
			name:            "memory limit",
			args:            []string{"--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath, "/bin/sleep", "5"},
			expectErrorCode: 137,
			rootfull:        true,
			rootless:        true,
		},
		{
			name:            "cpu success",
			args:            []string{"--apply-cgroups", "testdata/cgroups/cpu_success.toml", c.env.ImagePath, "/bin/true"},
			expectErrorCode: 0,
			rootfull:        true,
			// This currently fails in the e2e scenario due to the way we are using a mount namespace.
			// It *does* work if you test it, directly calling the apptainer CLI.
			// Reason is believed to be: https://github.com/opencontainers/runc/issues/3026
			rootless: false,
		},
		// Device access is allowed by default.
		{
			name:            "device allow default",
			args:            []string{"--apply-cgroups", "testdata/cgroups/null.toml", c.env.ImagePath, "cat", "/dev/null"},
			expectErrorCode: 0,
			rootfull:        true,
			rootless:        true,
		},
		// Device limits are properly applied only in rootful mode. Rootless will ignore them with a warning.
		{
			name:            "device deny",
			args:            []string{"--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"},
			expectErrorCode: 1,
			expectErrorOut:  "Operation not permitted",
			rootfull:        true,
			rootless:        false,
		},
		{
			name:            "device ignored",
			args:            []string{"--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"},
			expectErrorCode: 0,
			expectErrorOut:  "Device limits will not be applied with rootless cgroups",
			rootfull:        false,
			rootless:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if profile.Privileged() && !tt.rootfull {
				t.Skip()
			}
			if !profile.Privileged() && !tt.rootless {
				t.Skip()
			}
			exitFunc := []e2e.ApptainerCmdResultOp{}
			if tt.expectErrorOut != "" {
				exitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectError(e2e.ContainMatch, tt.expectErrorOut)}
			}
			c.env.RunApptainer(
				t,
				e2e.WithProfile(profile),
				e2e.WithCommand("exec"),
				e2e.WithArgs(tt.args...),
				e2e.ExpectExit(tt.expectErrorCode, exitFunc...),
			)
		})
	}
}

func (c *ctx) actionApplyRoot(t *testing.T) {
	c.actionApply(t, e2e.RootProfile)
}

func (c *ctx) actionApplyRootless(t *testing.T) {
	for _, profile := range []e2e.Profile{e2e.UserProfile, e2e.UserNamespaceProfile, e2e.FakerootProfile} {
		t.Run(profile.String(), func(t *testing.T) {
			c.actionApply(t, profile)
		})
	}
}

type resourceFlagTest struct {
	name            string
	args            []string
	expectErrorCode int
	// cgroupsV1 - cgroupfs controller/resource to check, and content we expect to see
	controllerV1 string
	resourceV1   string
	expectV1     string
	// cgroupsV2 - delegation required when rootless
	delegationV2 string
	// cgroupsV2 - resource to check, and content we expect to see
	resourceV2 string
	expectV2   string
	skipV2     bool
}

var resourceFlagTests = []resourceFlagTest{
	{
		name:            "blkio-weight",
		args:            []string{"--blkio-weight", "50"},
		expectErrorCode: 0,
		controllerV1:    "blkio",
		// Could be `blkio.bfq.weight` if bfq is available. However, under
		// cgroups v1 older crun will not set blkio.bfq.weight, so only test
		// with blkio.weight.
		// Ref: https://github.com/containers/crun/issues/1157
		resourceV1:   "blkio.weight",
		expectV1:     "50",
		delegationV2: "io",
		resourceV2:   "io.bfq.weight",
		expectV2:     "default 50",
	},
	{
		name:            "cpus",
		args:            []string{"--cpus", "0.5"},
		expectErrorCode: 0,
		// 0.5 cpus = quota of 50000 with default period 100000
		controllerV1: "cpu",
		resourceV1:   "cpu.cfs_quota_us",
		expectV1:     "50000",
		delegationV2: "cpu",
		resourceV2:   "cpu.max",
		expectV2:     "50000 100000",
	},
	{
		name:            "cpu-shares",
		args:            []string{"--cpu-shares", "123"},
		expectErrorCode: 0,
		controllerV1:    "cpu",
		resourceV1:      "cpu.shares",
		expectV1:        "123",
		// Cgroups v2 has a conversion from shares to weight
		// weight = (1 + ((cpuShares-2)*9999)/262142)
		delegationV2: "cpu",
		resourceV2:   "cpu.weight",
		expectV2:     "5",
	},
	{
		name:            "cpuset-cpus",
		args:            []string{"--cpuset-cpus", "0", "--cpuset-mems", "0"},
		expectErrorCode: 0,
		controllerV1:    "cpuset",
		resourceV1:      "cpuset.cpus",
		expectV1:        "0",
		delegationV2:    "cpuset",
		resourceV2:      "cpuset.cpus",
		expectV2:        "0",
	},
	{
		name:            "cpuset-mems",
		args:            []string{"--cpuset-cpus", "0", "--cpuset-mems", "0"},
		expectErrorCode: 0,
		controllerV1:    "cpuset",
		resourceV1:      "cpuset.mems",
		expectV1:        "0",
		delegationV2:    "cpuset",
		resourceV2:      "cpuset.mems",
		expectV2:        "0",
	},
	{
		name:            "memory",
		args:            []string{"--memory", "500M"},
		expectErrorCode: 0,
		controllerV1:    "memory",
		resourceV1:      "memory.limit_in_bytes",
		expectV1:        "524288000",
		delegationV2:    "memory",
		resourceV2:      "memory.max",
		expectV2:        "524288000",
	},
	{
		name:            "memory-reservation",
		args:            []string{"--memory-reservation", "500M"},
		expectErrorCode: 0,
		controllerV1:    "memory",
		resourceV1:      "memory.soft_limit_in_bytes",
		expectV1:        "524288000",
		delegationV2:    "memory",
		resourceV2:      "memory.low",
		expectV2:        "524288000",
	},
	{
		// The CLI memory-swap value is v1 memory + swap... so this means 250M of swap
		name:            "memory-swap",
		args:            []string{"--memory-swap", "500M", "--memory", "250M"},
		expectErrorCode: 0,
		controllerV1:    "memory",
		resourceV1:      "memory.memsw.limit_in_bytes",
		// V1 shows the 500M combined
		expectV1: "524288000",
		// V2 treats the mem & swap separately... shows only 250M of swap (500M memory-swap - 250M memory)
		delegationV2: "memory",
		resourceV2:   "memory.swap.max",
		expectV2:     "262144000",
	},
	{
		name:            "oom-kill-disable",
		args:            []string{"--oom-kill-disable"},
		expectErrorCode: 0,
		controllerV1:    "memory",
		resourceV1:      "memory.oom_control",
		expectV1:        "oom_kill_disable 1",
		// v2 relies on oom_score_adj on /proc/pid instead
		skipV2: true,
	},
	{
		name:            "pids-limit",
		args:            []string{"--pids-limit", "123"},
		expectErrorCode: 0,
		controllerV1:    "pids",
		resourceV1:      "pids.max",
		expectV1:        "123",
		delegationV2:    "pids",
		resourceV2:      "pids.max",
		expectV2:        "123",
	},
}

func (c *ctx) actionFlags(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	for _, tt := range resourceFlagTests {
		t.Run(tt.name, func(t *testing.T) {
			if cgroups.IsCgroup2UnifiedMode() {
				c.actionFlagV2(t, tt, profile)
				return
			}
			c.actionFlagV1(t, tt, profile)
		})
	}
}

func (c *ctx) actionFlagV1(t *testing.T, tt resourceFlagTest, profile e2e.Profile) {
	// Don't try to test a resource that doesn't exist in our caller cgroup.
	// E.g. some systems don't have memory.memswp, and might not have blkio.bfq
	require.CgroupsResourceExists(t, tt.controllerV1, tt.resourceV1)

	// Use shell in the container to find container cgroup and cat the value for the tested controller & resource.
	// /proc/self/cgroup is : delimited
	// controller is the 2nd field in `/proc/self/cgroup`
	// cgroup path relative to root cgroup mount is the 3rd field in `/proc/self/cgroup`
	shellCmd := fmt.Sprintf("cat /sys/fs/cgroup/%s$(cat /proc/self/cgroup | grep '[,:]%s[,:]' | cut -d ':' -f 3)/%s", tt.controllerV1, tt.controllerV1, tt.resourceV1)

	exitFunc := []e2e.ApptainerCmdResultOp{}
	if tt.expectV1 != "" {
		exitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectOutput(e2e.ContainMatch, tt.expectV1)}
	}

	args := tt.args
	args = append(args, "-B", "/sys/fs/cgroup", c.env.ImagePath, "/bin/sh", "-c", shellCmd)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(args...),
		e2e.ExpectExit(tt.expectErrorCode, exitFunc...),
	)
}

func (c *ctx) actionFlagV2(t *testing.T, tt resourceFlagTest, profile e2e.Profile) {
	if tt.skipV2 {
		t.Skip()
	}
	// Don't try to test a resource that doesn't exist in our caller cgroup.
	// E.g. some systems don't have io.bfq.*
	require.CgroupsResourceExists(t, "", tt.resourceV2)

	// In rootless mode, can only test subsystems that have been delegated
	if !profile.Privileged() {
		require.CgroupsV2Delegated(t, tt.delegationV2)
	}

	exitFunc := []e2e.ApptainerCmdResultOp{}
	if tt.expectV2 != "" {
		exitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectOutput(e2e.ContainMatch, tt.expectV2)}
	}

	// Use shell in the container to find container cgroup and cat the value for the tested controller & resource.
	// /proc/self/cgroup is : delimited
	// For V2 the controller is null (field 2), at index 0 (field 1)
	// cgroup path relative to root cgroup mount is the 3rd field in `/proc/self/cgroup`
	shellCmd := fmt.Sprintf("cat /sys/fs/cgroup$(cat /proc/self/cgroup | grep '^0::' | cut -d ':' -f 3)/%s", tt.resourceV2)

	args := tt.args
	args = append(args, "-B", "/sys/fs/cgroup", c.env.ImagePath, "/bin/sh", "-c", shellCmd)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(args...),
		e2e.ExpectExit(tt.expectErrorCode, exitFunc...),
	)
}

func (c *ctx) actionFlagsRoot(t *testing.T) {
	c.actionFlags(t, e2e.RootProfile)
}

func (c *ctx) actionFlagsRootless(t *testing.T) {
	for _, profile := range []e2e.Profile{e2e.UserProfile, e2e.UserNamespaceProfile, e2e.FakerootProfile} {
		t.Run(profile.String(), func(t *testing.T) {
			c.actionFlags(t, profile)
		})
	}
}

func (c *ctx) instanceFlags(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	for _, tt := range resourceFlagTests {
		t.Run(tt.name, func(t *testing.T) {
			if cgroups.IsCgroup2UnifiedMode() {
				c.instanceFlagV2(t, tt, profile)
				return
			}
			c.instanceFlagV1(t, tt, profile)
		})
	}
}

func (c *ctx) instanceFlagV1(t *testing.T, tt resourceFlagTest, profile e2e.Profile) {
	// Don't try to test a resource that doesn't exist in our caller cgroup.
	// E.g. some systems don't have memory.memswp, and might not have blkio.bfq
	require.CgroupsResourceExists(t, tt.controllerV1, tt.resourceV1)

	instanceName := randomName(t)
	joinName := fmt.Sprintf("instance://%s", instanceName)
	startArgs := append(tt.args, "-B", "/sys/fs/cgroup", c.env.ImagePath, instanceName)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("start"),
		e2e.WithProfile(profile),
		e2e.WithCommand("instance start"),
		e2e.WithArgs(startArgs...),
		e2e.ExpectExit(0),
	)

	// Use shell in the container to find container cgroup and cat the value for the tested controller & resource.
	// /proc/self/cgroup is : delimited
	// controller is the 2nd field in `/proc/self/cgroup`
	// cgroup path relative to root cgroup mount is the 3rd field in `/proc/self/cgroup`
	shellCmd := fmt.Sprintf("cat /sys/fs/cgroup/%s$(cat /proc/self/cgroup | grep '[,:]%s[,:]' | cut -d ':' -f 3)/%s", tt.controllerV1, tt.controllerV1, tt.resourceV1)
	exitFunc := []e2e.ApptainerCmdResultOp{}
	if tt.expectV1 != "" {
		exitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectOutput(e2e.ContainMatch, tt.expectV1)}
	}

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("exec"),
		e2e.WithProfile(profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(joinName, "/bin/sh", "-c", shellCmd),
		e2e.WithDir(profile.HostUser(t).Dir),
		e2e.ExpectExit(tt.expectErrorCode, exitFunc...),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("stop"),
		e2e.WithProfile(profile),
		e2e.WithCommand("instance stop"),
		e2e.WithArgs(instanceName),
		e2e.ExpectExit(0),
	)
}

func (c *ctx) instanceFlagV2(t *testing.T, tt resourceFlagTest, profile e2e.Profile) {
	if tt.skipV2 {
		t.Skip()
	}
	// Don't try to test a resource that doesn't exist in our caller cgroup.
	// E.g. some systems don't have io.bfq.*
	require.CgroupsResourceExists(t, "", tt.resourceV2)

	// In rootless mode, can only test subsystems that have been delegated
	if !profile.Privileged() {
		require.CgroupsV2Delegated(t, tt.delegationV2)
	}

	instanceName := randomName(t)
	joinName := fmt.Sprintf("instance://%s", instanceName)
	startArgs := append(tt.args, "-B", "/sys/fs/cgroup", c.env.ImagePath, instanceName)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("start"),
		e2e.WithProfile(profile),
		e2e.WithCommand("instance start"),
		e2e.WithArgs(startArgs...),
		e2e.ExpectExit(0),
	)

	// Use shell in the container to find container cgroup and cat the value for the tested controller & resource.
	// /proc/self/cgroup is : delimited
	// For V2 the controller is null (field 2), at index 0 (field 1)
	// cgroup path relative to root cgroup mount is the 3rd field in `/proc/self/cgroup`
	shellCmd := fmt.Sprintf("cat /sys/fs/cgroup$(cat /proc/self/cgroup | grep '^0::' | cut -d ':' -f 3)/%s", tt.resourceV2)
	exitFunc := []e2e.ApptainerCmdResultOp{}
	if tt.expectV2 != "" {
		exitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectOutput(e2e.ContainMatch, tt.expectV2)}
	}

	execProfile := profile
	if profile.String() == e2e.FakerootProfile.String() {
		execProfile = e2e.UserNamespaceProfile
	}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("exec"),
		e2e.WithProfile(execProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(joinName, "/bin/sh", "-c", shellCmd),
		e2e.WithDir(profile.HostUser(t).Dir),
		e2e.ExpectExit(tt.expectErrorCode, exitFunc...),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("stop"),
		e2e.WithProfile(profile),
		e2e.WithCommand("instance stop"),
		e2e.WithArgs(instanceName),
		e2e.ExpectExit(0),
	)
}

func (c *ctx) instanceFlagsRoot(t *testing.T) {
	c.instanceFlags(t, e2e.RootProfile)
}

func (c *ctx) instanceFlagsRootless(t *testing.T) {
	for _, profile := range []e2e.Profile{e2e.UserProfile, e2e.UserNamespaceProfile, e2e.FakerootProfile} {
		t.Run(profile.String(), func(t *testing.T) {
			c.instanceFlags(t, profile)
		})
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := &ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"instance stats root":             np(env.WithRootManagers(c.instanceStatsRoot)),
		"instance stats rootless":         np(env.WithRootlessManagers(c.instanceStatsRootless)),
		"instance root cgroups":           np(env.WithRootManagers(c.instanceApplyRoot)),
		"instance rootless cgroups":       np(env.WithRootlessManagers(c.instanceApplyRootless)),
		"instance flags root cgroups":     np(env.WithRootManagers(c.instanceFlagsRoot)),
		"instance flags rootless cgroups": np(env.WithRootlessManagers(c.instanceFlagsRootless)),
		"action root cgroups":             np(env.WithRootManagers(c.actionApplyRoot)),
		"action rootless cgroups":         np(env.WithRootlessManagers(c.actionApplyRootless)),
		"action flags root cgroups":       np(env.WithRootManagers(c.actionFlagsRoot)),
		"action flags rootless cgroups":   np(env.WithRootlessManagers(c.actionFlagsRootless)),
	}
}
