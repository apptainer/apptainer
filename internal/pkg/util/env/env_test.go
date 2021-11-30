// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package env

import (
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
)

func TestSetFromList(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tt := []struct {
		name    string
		environ []string
		wantErr bool
	}{
		{
			name: "all ok",
			environ: []string{
				"LD_LIBRARY_PATH=/.apptainer.d/libs",
				"HOME=/home/tester",
				"PS1=test",
				"TERM=xterm-256color",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"PWD=/tmp",
				"LC_ALL=C",
				"APPTAINER_NAME=lolcow.sif",
			},
			wantErr: false,
		},
		{
			name: "bad envs",
			environ: []string{
				"LD_LIBRARY_PATH=/.apptainer.d/libs",
				"HOME=/home/tester",
				"PS1=test",
				"TERM=xterm-256color",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"TEST",
				"LC_ALL=C",
				"APPTAINER_NAME=lolcow.sif",
			},
			wantErr: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := SetFromList(tc.environ)
			if tc.wantErr && err == nil {
				t.Fatalf("Expected error, but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
