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
	"context"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestHook(t *testing.T) {
	state := &specs.State{
		Version: specs.Version,
		ID:      "hooked",
		Status:  specs.StateCreating,
		Pid:     1,
		Bundle:  t.TempDir(),
	}
	env := []string{"PATH=/usr/bin:/bin"}

	tests := []struct {
		name    string
		hook    specs.Hook
		wantErr string
	}{
		{
			name: "state on stdin",
			hook: specs.Hook{Path: "/bin/sh", Args: []string{"sh", "-c", `grep -q '"id":"hooked"'`}, Env: env},
		},
		{
			name:    "failure reports stderr",
			hook:    specs.Hook{Path: "/bin/sh", Args: []string{"sh", "-c", "echo no such device >&2; exit 3"}, Env: env},
			wantErr: "exit status 3: no such device",
		},
		{
			name:    "missing hook",
			hook:    specs.Hook{Path: "/nonexistent/hook", Args: []string{"hook"}},
			wantErr: "failed to execute hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Hook(context.Background(), &tt.hook, state)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
