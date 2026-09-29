// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package rpm

import (
	"os/exec"
	"testing"
)

func TestGetMacro(t *testing.T) {
	_, err := exec.LookPath("rpm")
	if err != nil {
		t.Skipf("rpm command not found in $PATH")
	}

	tests := []struct {
		name      string
		macroName string
		wantValue string
		wantErr   error
	}{
		{
			name:      "_host_os",
			macroName: "_host_os",
			wantValue: "linux",
			wantErr:   nil,
		},
		{
			name:      "not defined",
			macroName: "_not_a_macro_abc_123",
			wantValue: "",
			wantErr:   ErrMacroUndefined,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, err := GetMacro(tt.macroName)
			if err != tt.wantErr {
				t.Errorf("GetMacro() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotValue != tt.wantValue {
				t.Errorf("GetMacro() = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestArch(t *testing.T) {
	tests := []struct {
		name            string
		platformArch    string
		platformVariant string
		want            string
	}{
		{
			name:         "amd64",
			platformArch: "amd64",
			want:         "x86_64",
		},
		{
			name:            "amd64_v2",
			platformArch:    "amd64",
			platformVariant: "v2",
			want:            "x86_64_v2",
		},
		{
			name:            "amd64_v3",
			platformArch:    "amd64",
			platformVariant: "v3",
			want:            "x86_64",
		},
		{
			name:         "arm64",
			platformArch: "arm64",
			want:         "aarch64",
		},
		{
			name:         "arm",
			platformArch: "arm",
			want:         "armv7hl",
		},
		{
			name:            "arm_v5",
			platformArch:    "arm",
			platformVariant: "v5",
			want:            "armv5hl",
		},
		{
			name:            "arm_v6",
			platformArch:    "arm",
			platformVariant: "v6",
			want:            "armv6hl",
		},
		{
			name:            "arm_v7",
			platformArch:    "arm",
			platformVariant: "v7",
			want:            "armv7hl",
		},
		{
			name:         "ppc64le",
			platformArch: "ppc64le",
			want:         "ppc64le",
		},
		{
			name:         "s390x",
			platformArch: "s390x",
			want:         "s390x",
		},
		{
			name:         "riscv64",
			platformArch: "riscv64",
			want:         "riscv64",
		},
		{
			name:         "loong64",
			platformArch: "loong64",
			want:         "loong64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Arch(tt.platformArch, tt.platformVariant)
			if got != tt.want {
				t.Errorf("Arch() = %v, want %v", got, tt.want)
			}
		})
	}
}
