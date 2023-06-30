// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/apptainer/apptainer/pkg/sylog"
)

func TestCreateConfDir(t *testing.T) {
	// create a random name for a directory
	// TODO - go 1.20 initializes seed randomly by default, so can drop this
	// deprecated call in future.
	rand.Seed(time.Now().UnixNano()) //nolint:staticcheck
	bytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		bytes[i] = byte(65 + rand.Intn(25))
	}
	dir := "/tmp/" + string(bytes)

	// create the directory and check that it exists
	handleConfDir(dir, "")
	defer os.RemoveAll(dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("failed to create directory %s", dir)
	} else {
		// stick something in the directory and make sure it isn't deleted
		os.WriteFile(dir+"/foo", []byte(""), 0o655)
		handleConfDir(dir, "")
		if _, err := os.Stat(dir + "/foo"); os.IsNotExist(err) {
			t.Errorf("inadvertently overwrote existing directory %s", dir)
		}
	}
}

func TestChangeLogLevelViaEnvVariables(t *testing.T) {
	tests := []struct {
		Name  string
		Envs  []string
		Level int
	}{
		{
			Name:  "silent with no color",
			Envs:  []string{"APPTAINER_SILENT", "APPTAINER_NOCOLOR"},
			Level: -3,
		},
		{
			Name:  "silent",
			Envs:  []string{"APPTAINER_SILENT"},
			Level: -3,
		},
		{
			Name:  "quiet with no color",
			Envs:  []string{"APPTAINER_QUIET", "APPTAINER_NOCOLOR"},
			Level: -1,
		},
		{
			Name:  "quiet",
			Envs:  []string{"APPTAINER_QUIET"},
			Level: -1,
		},
		{
			Name:  "verbose with no color",
			Envs:  []string{"APPTAINER_VERBOSE", "APPTAINER_NOCOLOR"},
			Level: 4,
		},
		{
			Name:  "verbose",
			Envs:  []string{"APPTAINER_VERBOSE"},
			Level: 4,
		},
		{
			Name:  "debug with no color",
			Envs:  []string{"APPTAINER_DEBUG", "APPTAINER_NOCOLOR"},
			Level: 5,
		},
		{
			Name:  "debug",
			Envs:  []string{"APPTAINER_DEBUG"},
			Level: 5,
		},
	}

	// initialize apptainerCmd
	Init(false)
	for _, test := range tests {
		t.Log("starting test:" + test.Name)
		for _, env := range test.Envs {
			err := os.Setenv(env, "1")
			if err != nil {
				t.Error(err)
			}
		}

		// call persistentPreRunE to update cmd
		err := apptainerCmd.PersistentPreRunE(apptainerCmd, []string{})
		if err != nil {
			t.Error(err)
		}

		if len(test.Envs) == 2 {
			sylog.SetLevel(test.Level, true)
		}

		if sylog.GetLevel() != test.Level {
			t.Errorf("actual log level: %d, expected log level: %d", sylog.GetLevel(), test.Level)
		}

		for _, env := range test.Envs {
			os.Unsetenv(env)
		}
	}
}
