// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/client/oras"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/internal/pkg/util/uri"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

var (
	// PushLibraryURI holds the base URI to a Sylabs library API instance
	PushLibraryURI string

	// unsignedPush when true will allow pushing a unsigned container
	unsignedPush bool

	// pushDescription holds a description to be set against a library container
	pushDescription string
)

// --library
var pushLibraryURIFlag = cmdline.Flag{
	ID:           "pushLibraryURIFlag",
	Value:        &PushLibraryURI,
	DefaultValue: "",
	Name:         "library",
	Usage:        "the library to push to",
	EnvKeys:      []string{"LIBRARY"},
}

// -U|--allow-unsigned
var pushAllowUnsignedFlag = cmdline.Flag{
	ID:           "pushAllowUnsignedFlag",
	Value:        &unsignedPush,
	DefaultValue: false,
	Name:         "allow-unsigned",
	ShortHand:    "U",
	Usage:        "do not require a signed container image",
	EnvKeys:      []string{"ALLOW_UNSIGNED"},
}

// -D|--description
var pushDescriptionFlag = cmdline.Flag{
	ID:           "pushDescriptionFlag",
	Value:        &pushDescription,
	DefaultValue: "",
	Name:         "description",
	ShortHand:    "D",
	Usage:        "description for container image (library:// only)",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(PushCmd)

		cmdManager.RegisterFlagForCmd(&pushLibraryURIFlag, PushCmd)
		cmdManager.RegisterFlagForCmd(&pushAllowUnsignedFlag, PushCmd)
		cmdManager.RegisterFlagForCmd(&pushDescriptionFlag, PushCmd)
		cmdManager.RegisterFlagForCmd(&commonNoHTTPSFlag, PushCmd)

		cmdManager.RegisterFlagForCmd(&dockerHostFlag, PushCmd)
		cmdManager.RegisterFlagForCmd(&dockerUsernameFlag, PushCmd)
		cmdManager.RegisterFlagForCmd(&dockerPasswordFlag, PushCmd)
	})
}

// PushCmd apptainer push
var PushCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		file, dest := args[0], args[1]

		transport, ref := uri.Split(dest)
		if transport == "" {
			sylog.Fatalf("bad uri %s", dest)
		}

		switch transport {
		case LibraryProtocol: // Handle pushing to a library
			lc, err := getLibraryClientConfig(PushLibraryURI)
			if err != nil {
				sylog.Fatalf("Unable to get library client configuration: %v", err)
			}

			// Push to library requires a valid authToken
			if lc.AuthToken == "" {
				sylog.Fatalf("Cannot push image to library: %v", remoteWarning)
			}

			co, err := getKeyserverClientOpts("", endpoint.KeyserverVerifyOp)
			if err != nil {
				sylog.Fatalf("Unable to get keyserver client configuration: %v", err)
			}
			feURL, err := currentRemoteEndpoint.GetURL()
			if err != nil {
				sylog.Fatalf("Unable to find remote web URI %v", err)
			}

			pushSpec := apptainer.LibraryPushSpec{
				SourceFile:    file,
				DestRef:       dest,
				Description:   pushDescription,
				AllowUnsigned: unsignedPush,
				FrontendURI:   feURL,
			}

			err = apptainer.LibraryPush(cmd.Context(), pushSpec, lc, co)
			if err == apptainer.ErrLibraryUnsigned {
				fmt.Printf("TIP: You can push unsigned images with 'apptainer push -U %s'.\n", file)
				fmt.Printf("TIP: Learn how to sign your own containers by using 'apptainer help sign'\n\n")
				sylog.Fatalf("Unable to upload container: unable to verify signature")
			} else if err != nil {
				sylog.Fatalf("Unable to push image to library: %v", err)
			}
		case OrasProtocol:
			if cmd.Flag(pushDescriptionFlag.Name).Changed {
				sylog.Warningf("Description is not supported for push to oras. Ignoring it.")
			}
			ociAuth, err := makeDockerCredentials(cmd)
			if err != nil {
				sylog.Fatalf("Unable to make docker oci credentials: %s", err)
			}

			if err := oras.UploadImage(cmd.Context(), file, ref, ociAuth, noHTTPS); err != nil {
				sylog.Fatalf("Unable to push image to oci registry: %v", err)
			}
			sylog.Infof("Upload complete")
		case "":
			sylog.Fatalf("Transport type URI required but not supplied")
		default:
			sylog.Fatalf("Unsupported transport type: %s", transport)
		}
	},

	Use:     docs.PushUse,
	Short:   docs.PushShort,
	Long:    docs.PushLong,
	Example: docs.PushExample,
}
