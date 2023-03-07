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
