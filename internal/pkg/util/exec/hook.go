// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Hook executes an OCI hook command and passes the state over stdin. The hook
// keeps the capabilities this process can hand over, its permitted set within
// its bounding set, raised into the ambient set so that they survive the exec
// when the process is not uid 0, as inside an unprivileged user namespace. A
// hook that fails has what it wrote to stderr reported.
func Hook(ctx context.Context, hook *specs.Hook, state *specs.State) error {
	var cancel context.CancelFunc
	var timeout time.Duration
	var cmd *exec.Cmd

	if hook.Timeout != nil {
		timeout = time.Duration(*hook.Timeout) * 1000 * time.Millisecond
	}

	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if ctx != nil {
		cmd = exec.CommandContext(ctx, hook.Path)
	} else {
		cmd = exec.Command(hook.Path)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %s", err)
	}

	caps, err := ambientCapabilities()
	if err != nil {
		return err
	}

	stderr, err := os.CreateTemp("", "hook-stderr-")
	if err != nil {
		return fmt.Errorf("failed to create hook stderr file: %s", err)
	}
	defer os.Remove(stderr.Name())
	defer stderr.Close()

	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = stderr
	cmd.Env = hook.Env
	cmd.Args = hook.Args
	cmd.SysProcAttr = &syscall.SysProcAttr{AmbientCaps: caps}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to execute hook %s: %s", hook.Path, err)
	}

	err = cmd.Wait()
	if err != nil {
		if output, _ := os.ReadFile(stderr.Name()); len(bytes.TrimSpace(output)) > 0 {
			return fmt.Errorf("hook execution failed: %s: %s", err, bytes.TrimSpace(output))
		}
		return fmt.Errorf("hook execution failed: %s", err)
	}

	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("hook time out")
	}

	return err
}

// ambientCapabilities lists the capabilities of the current process that a
// child can be handed: those in both its permitted and bounding sets.
func ambientCapabilities() ([]uintptr, error) {
	permitted, err := capabilities.GetProcessPermitted()
	if err != nil {
		return nil, err
	}
	bounding, err := capabilities.GetProcessBounding()
	if err != nil {
		return nil, err
	}
	caps := make([]uintptr, 0, len(capabilities.Map))
	for _, c := range capabilities.Map {
		if permitted&bounding&(uint64(1)<<c.Value) != 0 {
			caps = append(caps, uintptr(c.Value))
		}
	}
	return caps, nil
}
