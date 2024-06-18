// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(BuildConfigCmd)
	})
}

// BuildConfigCmd outputs a list of the compile-time parameters with which
// apptainer was compiled
var BuildConfigCmd = &cobra.Command{
	RunE: func(_ *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		return printParam(name)
	},
	DisableFlagsInUseLine: true,

	Hidden:  true,
	Args:    cobra.MaximumNArgs(1),
	Use:     "buildcfg [parameter]",
	Short:   "Output the currently set compile-time parameters",
	Example: "$ apptainer buildcfg",
}

func printParam(name string) error {
	params := []struct {
		name  string
		value string
	}{
		{"PACKAGE_NAME", buildcfg.PACKAGE_NAME},
		{"PACKAGE_VERSION", buildcfg.PACKAGE_VERSION},
		{"BUILDDIR", buildcfg.BUILDDIR},
		{"PREFIX", buildcfg.PREFIX},
		{"EXECPREFIX", buildcfg.EXECPREFIX},
		{"BINDIR", buildcfg.BINDIR},
		{"SBINDIR", buildcfg.SBINDIR},
		{"LIBEXECDIR", buildcfg.LIBEXECDIR},
		{"DATAROOTDIR", buildcfg.DATAROOTDIR},
		{"DATADIR", buildcfg.DATADIR},
		{"SYSCONFDIR", buildcfg.SYSCONFDIR},
		{"SHAREDSTATEDIR", buildcfg.SHAREDSTATEDIR},
		{"LOCALSTATEDIR", buildcfg.LOCALSTATEDIR},
		{"RUNSTATEDIR", buildcfg.RUNSTATEDIR},
		{"INCLUDEDIR", buildcfg.INCLUDEDIR},
		{"DOCDIR", buildcfg.DOCDIR},
		{"INFODIR", buildcfg.INFODIR},
		{"LIBDIR", buildcfg.LIBDIR},
		{"LOCALEDIR", buildcfg.LOCALEDIR},
		{"MANDIR", buildcfg.MANDIR},
		{"APPTAINER_CONFDIR", buildcfg.APPTAINER_CONFDIR},
		{"SESSIONDIR", buildcfg.SESSIONDIR},
		{"PLUGIN_ROOTDIR", buildcfg.PLUGIN_ROOTDIR},
		{"APPTAINER_CONF_FILE", buildcfg.APPTAINER_CONF_FILE},
		{"APPTAINER_SUID_INSTALL", fmt.Sprintf("%d", buildcfg.APPTAINER_SUID_INSTALL)},
	}

	if name != "" {
		for _, p := range params {
			if p.name == name {
				fmt.Println(p.value)
				return nil
			}
		}
		return fmt.Errorf("no variable named %q", name)
	}
	for _, p := range params {
		fmt.Printf("%s=%s\n", p.name, p.value)
	}
	return nil
}
