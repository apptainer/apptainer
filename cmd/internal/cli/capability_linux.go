// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"errors"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// CapConfig contains flag variables for capability commands
type CapConfig struct {
	CapUser  string
	CapGroup string
}

var capConfig = new(CapConfig)

// -u|--user
var capUserFlag = cmdline.Flag{
	ID:           "capUserFlag",
	Value:        &capConfig.CapUser,
	DefaultValue: "",
	Name:         "user",
	ShortHand:    "u",
	Usage:        "manage capabilities for a user",
	EnvKeys:      []string{"CAP_USER"},
}

// -g|--group
var capGroupFlag = cmdline.Flag{
	ID:           "capGroupFlag",
	Value:        &capConfig.CapGroup,
	DefaultValue: "",
	Name:         "group",
	ShortHand:    "g",
	Usage:        "manage capabilities for a group",
	EnvKeys:      []string{"CAP_GROUP"},
}

// CapabilityAvailCmd apptainer capability avail
var CapabilityAvailCmd = &cobra.Command{
	Args:                  cobra.RangeArgs(0, 1),
	DisableFlagsInUseLine: true,
	Run: func(_ *cobra.Command, args []string) {
		caps := ""
		if len(args) > 0 {
			caps = args[0]
		}
		c := apptainer.CapAvailConfig{
			Caps: caps,
			Desc: len(args) == 0,
		}
		if err := apptainer.CapabilityAvail(c); err != nil {
			sylog.Fatalf("Unable to list available capabilities: %s", err)
		}
	},

	Use:     docs.CapabilityAvailUse,
	Short:   docs.CapabilityAvailShort,
	Long:    docs.CapabilityAvailLong,
	Example: docs.CapabilityAvailExample,
}

// CapabilityAddCmd apptainer capability add
var CapabilityAddCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(_ *cobra.Command, args []string) {
		c := apptainer.CapManageConfig{
			Caps:  args[0],
			User:  capConfig.CapUser,
			Group: capConfig.CapGroup,
		}

		if err := apptainer.CapabilityAdd(buildcfg.CAPABILITY_FILE, c); err != nil {
			sylog.Fatalf("Unable to add capabilities: %s", err)
		}
	},

	Use:     docs.CapabilityAddUse,
	Short:   docs.CapabilityAddShort,
	Long:    docs.CapabilityAddLong,
	Example: docs.CapabilityAddExample,
}

// CapabilityDropCmd apptainer capability drop
var CapabilityDropCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(_ *cobra.Command, args []string) {
		c := apptainer.CapManageConfig{
			Caps:  args[0],
			User:  capConfig.CapUser,
			Group: capConfig.CapGroup,
		}

		if err := apptainer.CapabilityDrop(buildcfg.CAPABILITY_FILE, c); err != nil {
			sylog.Fatalf("Unable to drop capabilities: %s", err)
		}
	},

	Use:     docs.CapabilityDropUse,
	Short:   docs.CapabilityDropShort,
	Long:    docs.CapabilityDropLong,
	Example: docs.CapabilityDropExample,
}

// CapabilityListCmd apptainer capability list
var CapabilityListCmd = &cobra.Command{
	Args:                  cobra.RangeArgs(0, 1),
	DisableFlagsInUseLine: true,
	Run: func(_ *cobra.Command, args []string) {
		userGroup := ""
		if len(args) == 1 {
			userGroup = args[0]
		}
		c := apptainer.CapListConfig{
			User:  userGroup,
			Group: userGroup,
			All:   len(args) == 0,
		}

		if err := apptainer.CapabilityList(buildcfg.CAPABILITY_FILE, c); err != nil {
			sylog.Fatalf("Unable to list capabilities: %s", err)
		}
	},

	Use:     docs.CapabilityListUse,
	Short:   docs.CapabilityListShort,
	Long:    docs.CapabilityListLong,
	Example: docs.CapabilityListExample,
}

// CapabilityCmd is the capability command
var CapabilityCmd = &cobra.Command{
	RunE: func(_ *cobra.Command, _ []string) error {
		return errors.New("invalid command")
	},
	DisableFlagsInUseLine: true,

	Aliases:       []string{"caps"},
	Use:           docs.CapabilityUse,
	Short:         docs.CapabilityShort,
	Long:          docs.CapabilityLong,
	Example:       docs.CapabilityExample,
	SilenceErrors: true,
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(CapabilityCmd)

		cmdManager.RegisterSubCmd(CapabilityCmd, CapabilityAddCmd)
		cmdManager.RegisterSubCmd(CapabilityCmd, CapabilityDropCmd)
		cmdManager.RegisterSubCmd(CapabilityCmd, CapabilityListCmd)
		cmdManager.RegisterSubCmd(CapabilityCmd, CapabilityAvailCmd)

		cmdManager.RegisterFlagForCmd(&capUserFlag, CapabilityAddCmd, CapabilityDropCmd)
		cmdManager.RegisterFlagForCmd(&capGroupFlag, CapabilityAddCmd, CapabilityDropCmd)
	})
}
