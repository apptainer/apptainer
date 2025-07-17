// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
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

var (
	keyserverInsecure bool
	keyserverOrder    uint32
)

// -c|--config
var keyserverConfigFlag = cmdline.Flag{
	ID:           "keyserverConfigFlag",
	Value:        &remoteConfig,
	DefaultValue: remoteConfigUser,
	Name:         "config",
	ShortHand:    "c",
	Usage:        "path to the file holding keyserver configurations",
}

// -i|--insecure
var keyserverInsecureFlag = cmdline.Flag{
	ID:           "keyserverInsecureFlag",
	Value:        &keyserverInsecure,
	DefaultValue: false,
	Name:         "insecure",
	ShortHand:    "i",
	Usage:        "allow insecure connection to keyserver",
}

// -o|--order
var keyserverOrderFlag = cmdline.Flag{
	ID:           "keyserverOrderFlag",
	Value:        &keyserverOrder,
	DefaultValue: uint32(0),
	Name:         "order",
	ShortHand:    "o",
	Usage:        "define the keyserver order",
}

// -u|--username
var keyserverLoginUsernameFlag = cmdline.Flag{
	ID:           "keyserverLoginUsernameFlag",
	Value:        &loginUsername,
	DefaultValue: "",
	Name:         "username",
	ShortHand:    "u",
	Usage:        "username to authenticate with (required for Docker/OCI registry login)",
	EnvKeys:      []string{"LOGIN_USERNAME"},
}

// -p|--password
var keyserverLoginPasswordFlag = cmdline.Flag{
	ID:           "keyserverLoginPasswordFlag",
	Value:        &loginPassword,
	DefaultValue: "",
	Name:         "password",
	ShortHand:    "p",
	Usage:        "password / token to authenticate with",
	EnvKeys:      []string{"LOGIN_PASSWORD"},
}

// --password-stdin
var keyserverLoginPasswordStdinFlag = cmdline.Flag{
	ID:           "keyserverLoginPasswordStdinFlag",
	Value:        &loginPasswordStdin,
	DefaultValue: false,
	Name:         "password-stdin",
	Usage:        "take password from standard input",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(KeyserverCmd)
		cmdManager.RegisterSubCmd(KeyserverCmd, KeyserverAddCmd)
		cmdManager.RegisterSubCmd(KeyserverCmd, KeyserverRemoveCmd)
		cmdManager.RegisterSubCmd(KeyserverCmd, KeyserverLoginCmd)
		cmdManager.RegisterSubCmd(KeyserverCmd, KeyserverLogoutCmd)
		cmdManager.RegisterSubCmd(KeyserverCmd, KeyserverListCmd)

		// default location of the remote.yaml file is the user directory
		cmdManager.RegisterFlagForCmd(&keyserverConfigFlag, KeyserverCmd)

		cmdManager.RegisterFlagForCmd(&keyserverOrderFlag, KeyserverAddCmd)
		cmdManager.RegisterFlagForCmd(&keyserverInsecureFlag, KeyserverAddCmd)

		cmdManager.RegisterFlagForCmd(&keyserverLoginUsernameFlag, KeyserverLoginCmd)
		cmdManager.RegisterFlagForCmd(&keyserverLoginPasswordFlag, KeyserverLoginCmd)
		cmdManager.RegisterFlagForCmd(&keyserverLoginPasswordStdinFlag, KeyserverLoginCmd)
	})
}

// KeyserverCmd apptainer keyserver [...]
var KeyserverCmd = &cobra.Command{
	Run: nil,

	Use:     docs.KeyserverUse,
	Short:   docs.KeyserverShort,
	Long:    docs.KeyserverLong,
	Example: docs.KeyserverExample,

	DisableFlagsInUseLine: true,
}

// KeyserverAddCmd apptainer keyserver add [option] <keyserver_url>
var KeyserverAddCmd = &cobra.Command{
	Args:   cobra.RangeArgs(1, 2),
	PreRun: setKeyserver,
	Run: func(cmd *cobra.Command, args []string) {
		uri := args[0]
		name := ""
		if len(args) > 1 {
			name = args[0]
			uri = args[1]
		}

		if cmd.Flag(keyserverOrderFlag.Name).Changed && keyserverOrder == 0 {
			sylog.Fatalf("order must be > 0")
		}

		if err := apptainer.KeyserverAdd(name, uri, keyserverOrder, keyserverInsecure); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.KeyserverAddUse,
	Short:   docs.KeyserverAddShort,
	Long:    docs.KeyserverAddLong,
	Example: docs.KeyserverAddExample,

	DisableFlagsInUseLine: true,
}

// KeyserverRemoveCmd apptainer keyserver remove [remoteName] <keyserver_url>
var KeyserverRemoveCmd = &cobra.Command{
	Args:   cobra.RangeArgs(1, 2),
	PreRun: setKeyserver,
	Run: func(_ *cobra.Command, args []string) {
		uri := args[0]
		name := ""
		if len(args) > 1 {
			name = args[0]
			uri = args[1]
		}

		if err := apptainer.KeyserverRemove(name, uri); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.KeyserverRemoveUse,
	Short:   docs.KeyserverRemoveShort,
	Long:    docs.KeyserverRemoveLong,
	Example: docs.KeyserverRemoveExample,

	DisableFlagsInUseLine: true,
}

func setKeyserver(_ *cobra.Command, _ []string) {
	if os.Getuid() != 0 {
		sylog.Fatalf("Unable to modify keyserver configuration: not root user")
	}
}

// KeyserverLoginCmd apptainer keyserver login [option] <registry_url>
var KeyserverLoginCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		if err := apptainer.KeyserverLogin(remoteConfig, ObtainLoginArgs(args[0])); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.KeyserverLoginUse,
	Short:   docs.KeyserverLoginShort,
	Long:    docs.KeyserverLoginLong,
	Example: docs.KeyserverLoginExample,

	DisableFlagsInUseLine: true,
}

// KeyserverLogoutCmd apptainer keyserver logout [remoteName|serviceURI]
var KeyserverLogoutCmd = &cobra.Command{
	Args: cobra.RangeArgs(0, 1),
	Run: func(_ *cobra.Command, args []string) {
		// default to empty string to signal to KeyserverLogin to use default remote
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		if err := apptainer.KeyserverLogout(remoteConfig, name, reqAuthFile); err != nil {
			sylog.Fatalf("%s", err)
		}
		sylog.Infof("Logout succeeded")
	},

	Use:     docs.KeyserverLogoutUse,
	Short:   docs.KeyserverLogoutShort,
	Long:    docs.KeyserverLogoutLong,
	Example: docs.KeyserverLogoutExample,

	DisableFlagsInUseLine: true,
}

// KeyserverListCmd apptainer keyserver list
var KeyserverListCmd = &cobra.Command{
	Args: cobra.RangeArgs(0, 1),
	Run: func(_ *cobra.Command, args []string) {
		remoteName := ""
		if len(args) > 0 {
			remoteName = args[0]
		}
		if err := apptainer.KeyserverList(remoteName, remoteConfig); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.KeyserverListUse,
	Short:   docs.KeyserverListShort,
	Long:    docs.KeyserverListLong,
	Example: docs.KeyserverListExample,

	DisableFlagsInUseLine: true,
}
