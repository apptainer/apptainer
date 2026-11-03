// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2026, Apptainer a Series of LF Projects LLC. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cosign

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// GenerateKeyPair runs cosign generate-key-pair with the specified password
func GenerateKeyPair(ctx context.Context, password string) error {
	cmdArgs := []string{"generate-key-pair"}

	cmd := exec.CommandContext(ctx, "cosign", cmdArgs...)
	cmd.Dir = syfs.DefaultLocalKeyDirPath()
	if password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("COSIGN_PASSWORD=%s", password))
	} else {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign generate-key-pair failed: %w", err)
	}
	return nil
}

// Sign runs cosign sign with the specified reference
func Sign(ctx context.Context, ref string) error {
	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")
	cmdArgs := []string{"sign", "--key", "cosign.key", ref}

	cmd := exec.CommandContext(ctx, "cosign", cmdArgs...)
	cmd.Dir = syfs.DefaultLocalKeyDirPath()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sylog.Debugf("Executing %v", cmd.String())
	return cmd.Run()
}

// Verify runs cosign verify with the specified reference
func Verify(ctx context.Context, ref string, show bool) error {
	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")
	cmdArgs := []string{"verify", "--key", "cosign.pub", ref}

	cmd := exec.CommandContext(ctx, "cosign", cmdArgs...)
	cmd.Dir = syfs.DefaultLocalKeyDirPath()
	if show {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	sylog.Debugf("Executing %v", cmd.String())
	return cmd.Run()
}
