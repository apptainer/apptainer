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
	"os"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

var (
	cacheListTypes   []string
	cacheListVerbose bool
)

// -T|--type
var cacheListTypesFlag = cmdline.Flag{
	ID:           "cacheListTypes",
	Value:        &cacheListTypes,
	DefaultValue: []string{"all"},
	Name:         "type",
	ShortHand:    "T",
	Usage:        "a list of cache types to display, possible entries: library, oci, shub, blob(s), all",
}

// -s|--summary
var cacheListVerboseFlag = cmdline.Flag{
	ID:           "cacheListVerbose",
	Value:        &cacheListVerbose,
	DefaultValue: false,
	Name:         "verbose",
	ShortHand:    "v",
	Usage:        "include cache entries in the output",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&cacheListTypesFlag, CacheListCmd)
		cmdManager.RegisterFlagForCmd(&cacheListVerboseFlag, CacheListCmd)
	})
}

// CacheListCmd is 'apptainer cache list' and will list your local apptainer cache
var CacheListCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Run: func(_ *cobra.Command, _ []string) {
		if err := cacheListCmd(); err != nil {
			os.Exit(2)
		}
	},

	Use:     docs.CacheListUse,
	Short:   docs.CacheListShort,
	Long:    docs.CacheListLong,
	Example: docs.CacheListExample,
}

func cacheListCmd() error {
	// A get a handle for the current image cache
	imgCache := getCacheHandle(cache.Config{})
	if imgCache == nil {
		sylog.Fatalf("failed to create image cache handle")
	}

	err := apptainer.ListApptainerCache(imgCache, cacheListTypes, cacheListVerbose)
	if err != nil {
		sylog.Fatalf("An error occurred while listing cache: %v", err)
		return err
	}
	return nil
}
