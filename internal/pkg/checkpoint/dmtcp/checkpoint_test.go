// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package dmtcp

import "testing"

func TestValidateCheckpointName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty", value: "", wantErr: true},
		{name: "current directory", value: ".", wantErr: true},
		{name: "parent directory", value: "..", wantErr: true},
		{name: "parent traversal", value: "../checkpoint", wantErr: true},
		{name: "nested parent traversal", value: "../../checkpoint", wantErr: true},
		{name: "child path", value: "checkpoint/state", wantErr: true},
		{name: "relative path", value: "./checkpoint", wantErr: true},
		{name: "absolute path", value: "/tmp/checkpoint", wantErr: true},
		{name: "root path", value: "/", wantErr: true},
		{name: "normalized traversal", value: "checkpoint/../other", wantErr: true},
		{name: "simple name", value: "checkpoint", wantErr: false},
		{name: "name with dash", value: "checkpoint-1", wantErr: false},
		{name: "hidden name", value: ".checkpoint", wantErr: false},
		{name: "dots in name", value: "checkpoint..backup", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCheckpointName(tt.value)
			if tt.wantErr && err == nil {
				t.Errorf("validateCheckpointName(%q) succeeded, want error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateCheckpointName(%q) returned unexpected error: %v", tt.value, err)
			}
		})
	}
}

func TestCheckpointManagerRejectsPathNames(t *testing.T) {
	m := checkpointManager{}
	tests := []struct {
		name  string
		value string
	}{
		{name: "parent traversal", value: "../checkpoint"},
		{name: "nested parent traversal", value: "../../checkpoint"},
		{name: "child path", value: "checkpoint/state"},
		{name: "absolute path", value: "/tmp/checkpoint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := m.Create(tt.value); err == nil {
				t.Errorf("Create(%q) succeeded, want error", tt.value)
			}

			if _, err := m.Get(tt.value); err == nil {
				t.Errorf("Get(%q) succeeded, want error", tt.value)
			}

			if err := m.Delete(tt.value); err == nil {
				t.Errorf("Delete(%q) succeeded, want error", tt.value)
			}
		})
	}
}
