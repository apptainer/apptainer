// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// -c|--config
var registryConfigFlag = cmdline.Flag{
	ID:           "registryConfigFlag",
	Value:        &remoteConfig,
	DefaultValue: remoteConfigUser,
	Name:         "config",
	ShortHand:    "c",
	Usage:        "path to the file holding registry configurations",
}

// -u|--username
var registryLoginUsernameFlag = cmdline.Flag{
	ID:           "registryLoginUsernameFlag",
	Value:        &loginUsername,
	DefaultValue: "",
	Name:         "username",
	ShortHand:    "u",
	Usage:        "username to authenticate with (required for Docker/OCI registry login)",
	EnvKeys:      []string{"LOGIN_USERNAME"},
}

// -p|--password
var registryLoginPasswordFlag = cmdline.Flag{
	ID:           "registryLoginPasswordFlag",
	Value:        &loginPassword,
	DefaultValue: "",
	Name:         "password",
	ShortHand:    "p",
	Usage:        "password / token to authenticate with",
	EnvKeys:      []string{"LOGIN_PASSWORD"},
}

// --password-stdin
var registryLoginPasswordStdinFlag = cmdline.Flag{
	ID:           "registryLoginPasswordStdinFlag",
	Value:        &loginPasswordStdin,
	DefaultValue: false,
	Name:         "password-stdin",
	Usage:        "take password from standard input",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(RegistryCmd)
		cmdManager.RegisterSubCmd(RegistryCmd, RegistryLoginCmd)
		cmdManager.RegisterSubCmd(RegistryCmd, RegistryLogoutCmd)
		cmdManager.RegisterSubCmd(RegistryCmd, RegistryListCmd)

		// default location of the remote.yaml file is the user directory
		cmdManager.RegisterFlagForCmd(&registryConfigFlag, RegistryCmd)

		cmdManager.RegisterFlagForCmd(&registryLoginUsernameFlag, RegistryLoginCmd)
		cmdManager.RegisterFlagForCmd(&registryLoginPasswordFlag, RegistryLoginCmd)
		cmdManager.RegisterFlagForCmd(&registryLoginPasswordStdinFlag, RegistryLoginCmd)
	})
}

// RegistryCmd apptainer registry [...]
var RegistryCmd = &cobra.Command{
	Run: nil,

	Use:     docs.RegistryUse,
	Short:   docs.RegistryShort,
	Long:    docs.RegistryLong,
	Example: docs.RegistryExample,

	DisableFlagsInUseLine: true,
}

// RegistryLoginCmd apptainer registry login [option] <registry_url>
var RegistryLoginCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := apptainer.RegistryLogin(remoteConfig, ObtainLoginArgs(args[0])); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.RegistryLoginUse,
	Short:   docs.RegistryLoginShort,
	Long:    docs.RegistryLoginLong,
	Example: docs.RegistryLoginExample,

	DisableFlagsInUseLine: true,
}

// RegistryLogoutCmd apptainer remote logout [remoteName|serviceURI]
var RegistryLogoutCmd = &cobra.Command{
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		// default to empty string to signal to registryLogin to use default remote
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		if err := apptainer.RegistryLogout(remoteConfig, name); err != nil {
			sylog.Fatalf("%s", err)
		}
		sylog.Infof("Logout succeeded")
	},

	Use:     docs.RegistryLogoutUse,
	Short:   docs.RegistryLogoutShort,
	Long:    docs.RegistryLogoutLong,
	Example: docs.RegistryLogoutExample,

	DisableFlagsInUseLine: true,
}

// RegistryListCmd apptainer remote list
var RegistryListCmd = &cobra.Command{
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if err := apptainer.RegistryList(remoteConfig); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.RegistryListUse,
	Short:   docs.RegistryListShort,
	Long:    docs.RegistryListLong,
	Example: docs.RegistryListExample,

	DisableFlagsInUseLine: true,
}
