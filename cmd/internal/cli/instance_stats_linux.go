// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Vanessa Sochat. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"os"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// Basic Design
// apptainer instance stats <name>
// apptainer instance stats --json <name>

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&instanceStatsUserFlag, instanceStatsCmd)
		cmdManager.RegisterFlagForCmd(&instanceStatsJSONFlag, instanceStatsCmd)
		cmdManager.RegisterFlagForCmd(&instanceStatsNoStreamFlag, instanceStatsCmd)
	})
}

// -u|--user
var instanceStatsUser string

var instanceStatsUserFlag = cmdline.Flag{
	ID:           "instanceStatsUserFlag",
	Value:        &instanceStatsUser,
	DefaultValue: "",
	Name:         "user",
	ShortHand:    "u",
	Usage:        "view stats for an instance belonging to a user (root only)",
	Tag:          "<username>",
	EnvKeys:      []string{"USER"},
}

// -j|--json
var instanceStatsJSON bool

var instanceStatsJSONFlag = cmdline.Flag{
	ID:           "instanceStatsJSONFlag",
	Value:        &instanceStatsJSON,
	DefaultValue: false,
	Name:         "json",
	ShortHand:    "j",
	Usage:        "output stats in json",
}

// --no-stream

var instanceStatsNoStream bool

var instanceStatsNoStreamFlag = cmdline.Flag{
	ID:           "instanceStatsNoStreamFlag",
	Value:        &instanceStatsNoStream,
	DefaultValue: false,
	Name:         "no-stream",
	Usage:        "disable streaming (live update) of instance stats",
}

// apptainer instance stats
var instanceStatsCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		uid := os.Getuid()

		// Root is required to look at stats for another user
		if instanceStatsUser != "" && uid != 0 {
			sylog.Fatalf("Only the root user can look at stats of a user's instance")
		}

		// Instance name is the only arg
		name := args[0]
		return apptainer.InstanceStats(cmd.Context(), name, instanceStatsUser, instanceStatsJSON, instanceStatsNoStream)
	},

	Use:     docs.InstanceStatsUse,
	Short:   docs.InstanceStatsShort,
	Long:    docs.InstanceStatsLong,
	Example: docs.InstanceStatsExample,
}
