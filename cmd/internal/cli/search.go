// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"runtime"

	"github.com/apptainer/apptainer/internal/pkg/client/oci"
	"github.com/apptainer/apptainer/internal/pkg/util/uri"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

var (
	// SearchRegistryURI holds the base URI to a Registry API instance
	SearchRegistryURI string
	// SearchArch holds the architecture for images to display in search results
	SearchArch string
	// SearchSigned is set true to only search for signed containers
	SearchSigned bool
)

// --library
var searchRegistryFlag = cmdline.Flag{
	ID:           "searchRegistryFlag",
	Value:        &SearchRegistryURI,
	DefaultValue: "",
	Name:         "library",
	Usage:        "URI for library to search",
	EnvKeys:      []string{"LIBRARY"},
}

// --arch
var searchArchFlag = cmdline.Flag{
	ID:           "searchArchFlag",
	Value:        &SearchArch,
	DefaultValue: runtime.GOARCH,
	Name:         "arch",
	Usage:        "architecture to search for",
	EnvKeys:      []string{"SEARCH_ARCH"},
}

// --signed
var searchSignedFlag = cmdline.Flag{
	ID:           "searchSignedFlag",
	Value:        &SearchSigned,
	DefaultValue: false,
	Name:         "signed",
	Usage:        "architecture to search for",
	EnvKeys:      []string{"SEARCH_SIGNED"},
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(SearchCmd)

		cmdManager.RegisterFlagForCmd(&searchRegistryFlag, SearchCmd)
		cmdManager.RegisterFlagForCmd(&searchArchFlag, SearchCmd)
		cmdManager.RegisterFlagForCmd(&searchSignedFlag, SearchCmd)
	})
}

// SearchCmd apptainer search
var SearchCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		searchFor := args[len(args)-1]
		transport, _ := uri.Split(SearchRegistryURI)
		switch transport {
		case oci.IsSupported(transport):
			fallthrough
		case ShubProtocol, OrasProtocol, HTTPProtocol, HTTPSProtocol:
			fallthrough
		default:
			sylog.Fatalf("Unsupported transport type: %s for image [%s]", transport, searchFor)
		}
	},

	Use:     docs.SearchUse,
	Short:   docs.SearchShort,
	Long:    docs.SearchLong,
	Example: docs.SearchExample,
}
