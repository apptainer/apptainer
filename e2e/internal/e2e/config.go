// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"golang.org/x/sys/unix"
)

func SetupDefaultConfig(t *testing.T, path string) {
	c, err := apptainerconf.Parse("")
	if err != nil {
		t.Fatalf("while generating apptainer configuration: %s", err)
	}

	// e2e tests should call the specific external binaries found/coonfigured in the build.
	// Set default external paths from build time values
	c.CryptsetupPath = buildcfg.CRYPTSETUP_PATH
	c.GoPath = buildcfg.GO_PATH
	c.LdconfigPath = buildcfg.LDCONFIG_PATH
	c.MksquashfsPath = buildcfg.MKSQUASHFS_PATH
	c.NvidiaContainerCliPath = buildcfg.NVIDIA_CONTAINER_CLI_PATH
	c.UnsquashfsPath = buildcfg.UNSQUASHFS_PATH

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
