// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

var (
	overlaySize       int
	overlayDirs       []string
	isOverlayFakeroot bool
	overlaySparse     bool
)

// -s|--size
var overlaySizeFlag = cmdline.Flag{
	ID:           "overlaySizeFlag",
	Value:        &overlaySize,
	DefaultValue: 64,
	Name:         "size",
	ShortHand:    "s",
	Usage:        "size of the EXT3 writable overlay in MiB",
}

// --sparse/-S
var overlaySparseFlag = cmdline.Flag{
	ID:           "overlaySparseFlag",
	Value:        &overlaySparse,
	DefaultValue: false,
	Name:         "sparse",
	ShortHand:    "S",
	Usage:        "create a sparse overlay",
	EnvKeys:      []string{"SPARSE"},
}

// --create-dir
var overlayCreateDirFlag = cmdline.Flag{
	ID:           "overlayCreateDirFlag",
	Value:        &overlayDirs,
	DefaultValue: []string{},
	Name:         "create-dir",
	Usage:        "directory to create as part of the overlay layout",
}

// --fakeroot
var overlayFakerootFlag = cmdline.Flag{
	ID:           "overlayFakerootFlag",
	Value:        &isOverlayFakeroot,
	DefaultValue: false,
	Name:         "fakeroot",
	ShortHand:    "f",
	Usage:        "make overlay layout usable by actions run with --fakeroot",
	EnvKeys:      []string{"FAKEROOT"},
}

// OverlayCreateCmd is the 'overlay create' command that allows to create writable overlay.
var OverlayCreateCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if err := apptainer.OverlayCreate(overlaySize, args[0], overlaySparse, isOverlayFakeroot, overlayDirs...); err != nil {
			sylog.Fatalf(err.Error())
		}
		return nil
	},
	DisableFlagsInUseLine: true,

	Use:     docs.OverlayCreateUse,
	Short:   docs.OverlayCreateShort,
	Long:    docs.OverlayCreateLong,
	Example: docs.OverlayCreateExample,
}
