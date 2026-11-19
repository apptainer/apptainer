// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"golang.org/x/sys/unix"
)

const (
	Findsquash  = "pstree $PPID|grep squashfuse"
	Findfuse2fs = "pstree $PPID|grep fuse2fs"
)

func SetupDefaultConfig(t *testing.T, path string) {
	c, err := apptainerconf.Parse("")
	if err != nil {
		t.Fatalf("while generating apptainer configuration: %s", err)
	}
	apptainerconf.SetCurrentConfig(c)
	apptainerconf.SetBinaryPath(buildcfg.LIBEXECDIR, true)

	c.AllowSetuidMountExtfs = true

	Privileged(func(t *testing.T) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("while creating apptainer configuration: %s", err)
		}

		if err := apptainerconf.Generate(f, "", c); err != nil {
			t.Fatalf("while generating apptainer configuration: %s", err)
		}

		f.Close()

		if err := unix.Mount(path, buildcfg.APPTAINER_CONF_FILE, "", unix.MS_BIND, ""); err != nil {
			t.Fatalf("while mounting %s to %s: %s", path, buildcfg.APPTAINER_CONF_FILE, err)
		}
	})(t)
}

func SetDirective(t *testing.T, env TestEnv, directive, value string) {
	env.RunApptainer(
		t,
		WithProfile(RootProfile),
		WithCommand("config global"),
		WithArgs("--set", directive, value),
		ExpectExit(0),
	)
}

// AppendDirective appends a raw "directive = value" line to apptainer.conf,
// for values that `config global --set` rejects.
func AppendDirective(t *testing.T, directive, value string) {
	Privileged(func(t *testing.T) {
		f, err := os.OpenFile(buildcfg.APPTAINER_CONF_FILE, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatalf("while opening apptainer configuration: %s", err)
		}
		defer f.Close()
		if _, err := fmt.Fprintf(f, "%s = %s\n", directive, value); err != nil {
			t.Fatalf("while appending to apptainer configuration: %s", err)
		}
	})(t)
}

func ResetDirective(t *testing.T, env TestEnv, directive string) {
	env.RunApptainer(
		t,
		WithProfile(RootProfile),
		WithCommand("config global"),
		WithArgs("--reset", directive),
		ExpectExit(0),
	)
}
