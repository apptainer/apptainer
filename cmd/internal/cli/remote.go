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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/remote"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

const (
	remoteWarning = "no authentication token, log in with `apptainer remote login`"
)

var (
	loginTokenFile          string
	loginUsername           string
	loginPassword           string
	remoteConfig            string
	remoteKeyserverOrder    uint32
	remoteKeyserverInsecure bool
	loginPasswordStdin      bool
	loginInsecure           bool
	remoteNoLogin           bool
	global                  bool
	remoteUseExclusive      bool
	remoteAddInsecure       bool
)

// assemble values of remoteConfig for user/sys locations
var remoteConfigUser = syfs.RemoteConf()

// -g|--global
var remoteGlobalFlag = cmdline.Flag{
	ID:           "remoteGlobalFlag",
	Value:        &global,
	DefaultValue: false,
	Name:         "global",
	ShortHand:    "g",
	Usage:        "edit the list of globally configured remote endpoints",
}

// -c|--config
var remoteConfigFlag = cmdline.Flag{
	ID:           "remoteConfigFlag",
	Value:        &remoteConfig,
	DefaultValue: remoteConfigUser,
	Name:         "config",
	ShortHand:    "c",
	Usage:        "path to the file holding remote endpoint configurations",
}

// --tokenfile
var remoteTokenFileFlag = cmdline.Flag{
	ID:           "remoteTokenFileFlag",
	Value:        &loginTokenFile,
	DefaultValue: "",
	Name:         "tokenfile",
	Usage:        "path to the file holding auth token for login (remote endpoints only)",
}

// --no-login
var remoteNoLoginFlag = cmdline.Flag{
	ID:           "remoteNoLoginFlag",
	Value:        &remoteNoLogin,
	DefaultValue: false,
	Name:         "no-login",
	Usage:        "skip automatic login step",
}

// -u|--username
var remoteLoginUsernameFlag = cmdline.Flag{
	ID:           "remoteLoginUsernameFlag",
	Value:        &loginUsername,
	DefaultValue: "",
	Name:         "username",
	ShortHand:    "u",
	Usage:        "username to authenticate with",
	EnvKeys:      []string{"LOGIN_USERNAME"},
}

// -p|--password
var remoteLoginPasswordFlag = cmdline.Flag{
	ID:           "remoteLoginPasswordFlag",
	Value:        &loginPassword,
	DefaultValue: "",
	Name:         "password",
	ShortHand:    "p",
	Usage:        "password / token to authenticate with",
	EnvKeys:      []string{"LOGIN_PASSWORD"},
}

// --password-stdin
var remoteLoginPasswordStdinFlag = cmdline.Flag{
	ID:           "remoteLoginPasswordStdinFlag",
	Value:        &loginPasswordStdin,
	DefaultValue: false,
	Name:         "password-stdin",
	Usage:        "take password from standard input",
}

// -i|--insecure
var remoteLoginInsecureFlag = cmdline.Flag{
	ID:           "remoteLoginInsecureFlag",
	Value:        &loginInsecure,
	DefaultValue: false,
	Name:         "insecure",
	ShortHand:    "i",
	Usage:        "allow insecure login",
	EnvKeys:      []string{"LOGIN_INSECURE"},
}

// -e|--exclusive
var remoteUseExclusiveFlag = cmdline.Flag{
	ID:           "remoteUseExclusiveFlag",
	Value:        &remoteUseExclusive,
	DefaultValue: false,
	Name:         "exclusive",
	ShortHand:    "e",
	Usage:        "set the endpoint as exclusive (root user only, imply --global)",
}

// -o|--order (deprecated)
var remoteKeyserverOrderFlag = cmdline.Flag{
	ID:           "remoteKeyserverOrderFlag",
	Value:        &remoteKeyserverOrder,
	DefaultValue: uint32(0),
	Name:         "order",
	ShortHand:    "o",
	Hidden:       true,
}

// -i|--insecure (deprecated)
var remoteKeyserverInsecureFlag = cmdline.Flag{
	ID:           "remoteKeyserverInsecureFlag",
	Value:        &remoteKeyserverInsecure,
	DefaultValue: false,
	Name:         "insecure",
	ShortHand:    "i",
	Hidden:       true,
}

// -i|--insecure
var remoteAddInsecureFlag = cmdline.Flag{
	ID:           "remoteAddInsecureFlag",
	Value:        &remoteAddInsecure,
	DefaultValue: false,
	Name:         "insecure",
	ShortHand:    "i",
	Usage:        "allow connection to an insecure http remote.",
	EnvKeys:      []string{"ADD_INSECURE"},
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(RemoteCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteAddCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteRemoveCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteUseCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteListCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteLoginCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteLogoutCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteStatusCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteAddKeyserverCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteRemoveKeyserverCmd)
		cmdManager.RegisterSubCmd(RemoteCmd, RemoteGetLoginPasswordCmd)

		// default location of the remote.yaml file is the user directory
		cmdManager.RegisterFlagForCmd(&remoteConfigFlag, RemoteCmd)
		// use tokenfile to log in to a remote
		cmdManager.RegisterFlagForCmd(&remoteTokenFileFlag, RemoteLoginCmd, RemoteAddCmd)
		// add --global flag to remote add/remove/use commands
		cmdManager.RegisterFlagForCmd(&remoteGlobalFlag, RemoteAddCmd, RemoteRemoveCmd, RemoteUseCmd)
		// add --no-login flag to add command
		cmdManager.RegisterFlagForCmd(&remoteNoLoginFlag, RemoteAddCmd)
		// add --insecure, --no-login flags to add command
		cmdManager.RegisterFlagForCmd(&remoteAddInsecureFlag, RemoteAddCmd)

		cmdManager.RegisterFlagForCmd(&remoteLoginUsernameFlag, RemoteLoginCmd)
		cmdManager.RegisterFlagForCmd(&remoteLoginPasswordFlag, RemoteLoginCmd)
		cmdManager.RegisterFlagForCmd(&remoteLoginPasswordStdinFlag, RemoteLoginCmd)
		cmdManager.RegisterFlagForCmd(&remoteLoginInsecureFlag, RemoteLoginCmd)

		cmdManager.RegisterFlagForCmd(&remoteUseExclusiveFlag, RemoteUseCmd)

		cmdManager.RegisterFlagForCmd(&remoteKeyserverOrderFlag, RemoteAddKeyserverCmd)
		cmdManager.RegisterFlagForCmd(&remoteKeyserverInsecureFlag, RemoteAddKeyserverCmd)
	})
}

// RemoteCmd apptainer remote [...]
var RemoteCmd = &cobra.Command{
	Run: nil,

	Use:     docs.RemoteUse,
	Short:   docs.RemoteShort,
	Long:    docs.RemoteLong,
	Example: docs.RemoteExample,

	DisableFlagsInUseLine: true,
}

// setGlobalRemoteConfig will assign the appropriate value to remoteConfig if the global flag is set
func setGlobalRemoteConfig(_ *cobra.Command, _ []string) {
	if !global {
		return
	}

	uid := uint32(os.Getuid())
	if uid != 0 {
		sylog.Fatalf("Unable to modify global endpoint configuration file: not root user")
	}

	// set remoteConfig value to the location of the global remote.yaml file
	remoteConfig = remote.SystemConfigPath
}

// RemoteGetLoginPasswordCmd apptainer remote get-login-password
var RemoteGetLoginPasswordCmd = &cobra.Command{
	DisableFlagsInUseLine: true,

	Use:     docs.RemoteGetLoginPasswordUse,
	Short:   docs.RemoteGetLoginPasswordShort,
	Long:    docs.RemoteGetLoginPasswordLong,
	Example: docs.RemoteGetLoginPasswordExample,

	Run: func(cmd *cobra.Command, args []string) {
		defaultConfig := ""

		config, err := getLibraryClientConfig(defaultConfig)
		if err != nil {
			sylog.Errorf("Error initializing config: %v", err)
		}

		password, err := apptainer.RemoteGetLoginPassword(config)
		if err != nil {
			sylog.Errorf("error: %v", err)
		}
		if password != "" {
			fmt.Println(password)
		}
	},
}

// RemoteAddCmd apptainer remote add [remoteName] [remoteURI]
var RemoteAddCmd = &cobra.Command{
	Args:   cobra.ExactArgs(2),
	PreRun: setGlobalRemoteConfig,
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		uri := args[1]

		localInsecure := remoteAddInsecure
		if strings.HasPrefix(uri, "https://") {
			sylog.Infof("--insecure ignored for https remote")
			localInsecure = false
		}

		if strings.HasPrefix(uri, "http://") && !localInsecure {
			sylog.Fatalf("http URI requires --insecure or APPTAINER_ADD_INSECURE=true")
		}

		if err := apptainer.RemoteAdd(remoteConfig, name, uri, global, localInsecure); err != nil {
			sylog.Fatalf("%s", err)
		}
		sylog.Infof("Remote %q added.", name)

		// ensure that this was not called with global flag, otherwise this will store the token in the
		// world readable config
		if global && !remoteNoLogin {
			sylog.Infof("Global option detected. Will not automatically log into remote.")
		} else if !remoteNoLogin {
			loginArgs := &apptainer.LoginArgs{
				Name:      name,
				Tokenfile: loginTokenFile,
			}
			if err := apptainer.RemoteLogin(remoteConfig, loginArgs); err != nil {
				sylog.Fatalf("%s", err)
			}
		}
	},

	Use:     docs.RemoteAddUse,
	Short:   docs.RemoteAddShort,
	Long:    docs.RemoteAddLong,
	Example: docs.RemoteAddExample,

	DisableFlagsInUseLine: true,
}

// RemoteRemoveCmd apptainer remote remove [remoteName]
var RemoteRemoveCmd = &cobra.Command{
	Args:   cobra.ExactArgs(1),
	PreRun: setGlobalRemoteConfig,
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := apptainer.RemoteRemove(remoteConfig, name); err != nil {
			sylog.Fatalf("%s", err)
		}
		sylog.Infof("Remote %q removed.", name)
	},

	Use:     docs.RemoteRemoveUse,
	Short:   docs.RemoteRemoveShort,
	Long:    docs.RemoteRemoveLong,
	Example: docs.RemoteRemoveExample,

	DisableFlagsInUseLine: true,
}

// RemoteUseCmd apptainer remote use [remoteName]
var RemoteUseCmd = &cobra.Command{
	Args:   cobra.ExactArgs(1),
	PreRun: setGlobalRemoteConfig,
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := apptainer.RemoteUse(remoteConfig, name, global, remoteUseExclusive); err != nil {
			sylog.Fatalf("%s", err)
		}
		sylog.Infof("Remote %q now in use.", name)
	},

	Use:     docs.RemoteUseUse,
	Short:   docs.RemoteUseShort,
	Long:    docs.RemoteUseLong,
	Example: docs.RemoteUseExample,

	DisableFlagsInUseLine: true,
}

// RemoteListCmd apptainer remote list
var RemoteListCmd = &cobra.Command{
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if err := apptainer.RemoteList(remoteConfig); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.RemoteListUse,
	Short:   docs.RemoteListShort,
	Long:    docs.RemoteListLong,
	Example: docs.RemoteListExample,

	DisableFlagsInUseLine: true,
}

// RemoteLoginCmd apptainer remote login [remoteName]
var RemoteLoginCmd = &cobra.Command{
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		loginArgs := new(apptainer.LoginArgs)

		// default to empty string to signal to RemoteLogin to use default remote
		if len(args) > 0 {
			loginArgs.Name = args[0]
		}

		loginArgs.Username = loginUsername
		loginArgs.Password = loginPassword
		loginArgs.Tokenfile = loginTokenFile
		loginArgs.Insecure = loginInsecure

		if loginPasswordStdin {
			p, err := io.ReadAll(os.Stdin)
			if err != nil {
				sylog.Fatalf("Failed to read password from stdin: %s", err)
			}
			loginArgs.Password = strings.TrimSuffix(string(p), "\n")
			loginArgs.Password = strings.TrimSuffix(loginArgs.Password, "\r")
		}

		if err := apptainer.RemoteLogin(remoteConfig, loginArgs); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.RemoteLoginUse,
	Short:   docs.RemoteLoginShort,
	Long:    docs.RemoteLoginLong,
	Example: docs.RemoteLoginExample,

	DisableFlagsInUseLine: true,
}

// RemoteLogoutCmd apptainer remote logout [remoteName|serviceURI]
var RemoteLogoutCmd = &cobra.Command{
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		// default to empty string to signal to RemoteLogin to use default remote
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		if err := apptainer.RemoteLogout(remoteConfig, name); err != nil {
			sylog.Fatalf("%s", err)
		}
		sylog.Infof("Logout succeeded")
	},

	Use:     docs.RemoteLogoutUse,
	Short:   docs.RemoteLogoutShort,
	Long:    docs.RemoteLogoutLong,
	Example: docs.RemoteLogoutExample,

	DisableFlagsInUseLine: true,
}

// RemoteStatusCmd apptainer remote status [remoteName]
var RemoteStatusCmd = &cobra.Command{
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		// default to empty string to signal to RemoteStatus to use default remote
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		if err := apptainer.RemoteStatus(remoteConfig, name); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.RemoteStatusUse,
	Short:   docs.RemoteStatusShort,
	Long:    docs.RemoteStatusLong,
	Example: docs.RemoteStatusExample,

	DisableFlagsInUseLine: true,
}

// RemoteAddKeyserverCmd apptainer remote add-keyserver (deprecated)
var RemoteAddKeyserverCmd = &cobra.Command{
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		sylog.Warningf("'remote add-keyserver' is deprecated and will be removed in a future release; running 'keyserver add'")
		keyserverInsecure = remoteKeyserverInsecure
		keyserverOrder = remoteKeyserverOrder
		KeyserverAddCmd.Run(cmd, args)
	},

	Use:    "add-keyserver",
	Hidden: true,
}

// RemoteAddKeyserverCmd apptainer remote remove-keyserver (deprecated)
var RemoteRemoveKeyserverCmd = &cobra.Command{
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		sylog.Warningf("'remote remove-keyserver' is deprecated and will be removed in a future release; running 'keyserver remove'")
		KeyserverRemoveCmd.Run(cmd, args)
	},

	Use:    "remove-keyserver",
	Hidden: true,
}
