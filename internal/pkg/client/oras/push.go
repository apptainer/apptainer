// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/google/go-containerregistry/pkg/authn"
)

// Push will push an oras image from the specified location
func Push(ctx context.Context, path, ref string, ociAuth *authn.AuthConfig, noHTTPS bool, reqAuthFile string) error {
	img, err := image.Init(path, false)
	if err != nil {
		return err
	}
	defer img.File.Close()

	var sif bool
	var arch string
	switch img.Type {
	case image.SIF:
		sif = true
		arch, err = sifArch(path)
		if err != nil {
			return err
		}
	default:
		sif = false
		arch = runtime.GOARCH
	}

	return UploadImage(ctx, path, ref, arch, sif, ociAuth, noHTTPS, reqAuthFile)
}

func sifArch(filename string) (string, error) {
	f, err := sif.LoadContainerFromPath(filename, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return "", fmt.Errorf("unable to open: %v: %w", filename, err)
	}
	defer f.UnloadContainer()

	arch := f.PrimaryArch()
	if arch == "unknown" {
		return arch, fmt.Errorf("unknown architecture in SIF file")
	}
	return arch, nil
}
