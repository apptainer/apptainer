// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the URIs of this project regarding your
// rights to use or distribute this software.

package main

import (
	"errors"
	"fmt"

	"github.com/apptainer/apptainer/pkg/cmdline"
	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
	clicallback "github.com/apptainer/apptainer/pkg/plugin/callback/cli"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin.
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "example.com/cli-plugin",
		Author:      "Apptainer Team",
		Version:     "0.1.0",
		Description: "This is a short example CLI plugin for Apptainer",
	},
	Callbacks: []pluginapi.Callback{
		(clicallback.Command)(callbackVersion),
		(clicallback.Command)(callbackVerify),
		(clicallback.Command)(callbackTestCmd),
	},
}

func callbackVersion(manager *cmdline.CommandManager) {
	versionCmd := manager.GetCmd("version")
	if versionCmd == nil {
		sylog.Warningf("Could not find version command")
		return
	}

	var test string
	manager.RegisterFlagForCmd(&cmdline.Flag{
		Value:        &test,
		DefaultValue: "this is a test flag from plugin",
		Name:         "test",
		Usage:        "some text to print",
		Hidden:       false,
	}, versionCmd)

	f := versionCmd.PreRun
	versionCmd.PreRun = func(c *cobra.Command, args []string) {
		fmt.Printf("test: %v\n", test)
		if f != nil {
			f(c, args)
		}
	}
}

func callbackVerify(manager *cmdline.CommandManager) {
	verifyCmd := manager.GetCmd("verify")
	if verifyCmd == nil {
		sylog.Warningf("Could not find verify command")
		return
	}

	var abort bool
	manager.RegisterFlagForCmd(&cmdline.Flag{
		Value:        &abort,
		DefaultValue: false,
		Name:         "abort",
		Usage:        "should the verify command be aborted?",
	}, verifyCmd)

	f := verifyCmd.PreRunE
	verifyCmd.PreRunE = func(c *cobra.Command, args []string) error {
		if f != nil {
			if err := f(c, args); err != nil {
				return err
			}
		}

		if abort {
			return errors.New("aborting verify from the plugin")
		}
		return nil
	}
}

func callbackTestCmd(manager *cmdline.CommandManager) {
	manager.RegisterCmd(&cobra.Command{
		DisableFlagsInUseLine: true,
		Args:                  cobra.MinimumNArgs(1),
		Use:                   "test-cmd [args ...]",
		Short:                 "Test test",
		Long:                  "Long test long test long test",
		Example:               "apptainer test-cmd my test",
		Run: func(_ *cobra.Command, args []string) {
			fmt.Println("test-cmd is printing args:", args)
		},
		TraverseChildren: true,
	})
}
