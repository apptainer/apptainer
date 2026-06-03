// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/client/oras"
	"github.com/apptainer/apptainer/internal/pkg/util/uri"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(tagImageCmd)
		cmdManager.RegisterFlagForCmd(&dockerHostFlag, tagImageCmd)
		cmdManager.RegisterFlagForCmd(&dockerUsernameFlag, tagImageCmd)
		cmdManager.RegisterFlagForCmd(&dockerPasswordFlag, tagImageCmd)
		cmdManager.RegisterFlagForCmd(&dockerLoginFlag, tagImageCmd)
		cmdManager.RegisterFlagForCmd(&commonNoHTTPSFlag, tagImageCmd)
	})
}

var tagImageCmd = &cobra.Command{
	Use:     docs.TagUse,
	Short:   docs.TagShort,
	Long:    docs.TagLong,
	Example: docs.TagExample,
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		image, tag := args[0], args[1]

		transport, ref := uri.Split(image)
		if transport == "" {
			sylog.Fatalf("Bad URI %s", image)
		}
		if tag == "" {
			sylog.Fatalf("Bad tag %s", tag)
		}

		switch transport {
		case OrasProtocol:
			ociAuth, err := makeOCICredentials(cmd)
			if err != nil {
				sylog.Fatalf("Unable to make docker oci credentials: %s", err)
			}

			if err := oras.TagImage(cmd.Context(), ref, tag, ociAuth, noHTTPS, reqAuthFile); err != nil {
				sylog.Fatalf("Unable to tag image in oci registry: %v", err)
			}

			sylog.Infof("Image %s tagged %q.", ref, tag)
		case "":
			sylog.Fatalf("No transport type URI supplied")
		default:
			sylog.Fatalf("Unsupported transport type: %s", transport)
		}
	},
}
