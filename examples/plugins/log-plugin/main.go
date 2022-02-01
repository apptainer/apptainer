// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"fmt"
	"log/syslog"
	"os"

	"github.com/apptainer/apptainer/pkg/cmdline"
	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
	clicallback "github.com/apptainer/apptainer/pkg/plugin/callback/cli"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "github.com/apptainer/apptainer/log-plugin",
		Author:      "Sylabs Team",
		Version:     "0.2.0",
		Description: "Log executed CLI commands to syslog",
	},
	Callbacks: []pluginapi.Callback{
		(clicallback.Command)(logCommand),
	},
}

func logCommand(manager *cmdline.CommandManager) {
	rootCmd := manager.GetRootCmd()

	// Keep track of an existing PreRunE so we can call it
	f := rootCmd.PersistentPreRunE

	// The log action is added as a PreRunE on the main `apptainer` root command
	// so we can log anything a user does with `apptainer`.
	rootCmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		uid := os.Getuid()
		gid := os.Getgid()
		command := c.Name()
		msg := fmt.Sprintf("UID=%d GID=%d COMMAND=%s ARGS=%v", uid, gid, command, args)

		// This logger never errors, only warns, if it fails to write to syslog
		w, err := syslog.New(syslog.LOG_INFO, "apptainer")
		if err != nil {
			sylog.Warningf("Could not create syslog: %v", err)
		} else {
			defer w.Close()
			if err := w.Info(msg); err != nil {
				sylog.Warningf("Could not write to syslog: %v", err)
			}
		}

		// Call any existing PreRunE
		if f != nil {
			return f(c, args)
		}

		return nil
	}
}
