// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package confgen

import (
	"bytes"
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

func TestGenConf(t *testing.T) {
	tmpl := "testdata/test_default.tmpl"
	tests := []struct {
		name            string
		confInPath      string
		confOutPath     string
		confCorrectPath string
	}{
		{"gen_new", "", "testdata/test_1.out", "testdata/test_1.out.correct"},
		{"gen_update", "testdata/test_2.in", "testdata/test_2.out", "testdata/test_2.out.correct"},
		{"gen_update_newvals", "testdata/test_3.in", "testdata/test_3.out", "testdata/test_3.out.correct"},
		{"gen_bind_path_options", "testdata/test_4.in", "testdata/test_4.out", "testdata/test_4.out.correct"},
	}

	for _, tt := range tests {
		t.Run(tt.name, test.WithoutPrivilege(func(t *testing.T) {
			defer os.Remove(tt.confOutPath)

			if err := genConf(tmpl, tt.confInPath, tt.confOutPath); err != nil {
				t.Fatalf("Unable to generate config: %v", err)
			}

			if eq, err := compareFile(tt.confOutPath, tt.confCorrectPath); err != nil {
				t.Fatalf("Unable to compare files: %v\n", err)
			} else if !eq {
				t.Fatalf("Output file %v does not match correct output %v\n", tt.confOutPath, tt.confCorrectPath)
			}
		}))
	}
}

func TestWarnTrustedBindPaths(t *testing.T) {
	tests := []struct {
		name       string
		directives apptainerconf.Directives
		want       bool
	}{
		{"Absent", apptainerconf.Directives{}, false},
		{"No", apptainerconf.Directives{"trusted bind paths": {"no"}}, false},
		{"Yes", apptainerconf.Directives{"trusted bind paths": {"yes"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := warnTrustedBindPaths(tt.directives); got != tt.want {
				t.Errorf("warnTrustedBindPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func compareFile(p1, p2 string) (bool, error) {
	f1, err := os.ReadFile(p1)
	if err != nil {
		return false, err
	}

	f2, err := os.ReadFile(p2)
	if err != nil {
		return false, err
	}

	return bytes.Equal(f1, f2), nil
}
