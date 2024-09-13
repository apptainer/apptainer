// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociplatform

import (
	"fmt"
	"runtime"

	"github.com/apptainer/apptainer/pkg/sylog"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// CheckImagePlatform ensures that an image reference satisfies the provided platform requirements.
func CheckImagePlatform(platform v1.Platform, img v1.Image) error {
	cf, err := img.ConfigFile()
	if err != nil {
		return err
	}

	if cf.Platform() == nil {
		sylog.Warningf("OCI image doesn't declare a platform. It may not be compatible with this system.")
		return nil
	}

	if cf.Platform().Satisfies(platform) {
		return nil
	}

	return fmt.Errorf("image (%s) does not satisfy required platform (%s)", cf.Platform(), platform)
}

func DefaultPlatform() (*ggcrv1.Platform, error) {
	os := runtime.GOOS
	arch := runtime.GOARCH
	variant := CPUVariant()

	if os != "linux" {
		return nil, fmt.Errorf("%q is not a valid platform OS for apptainer", runtime.GOOS)
	}

	arch, variant = normalizeArch(arch, variant)

	return &ggcrv1.Platform{
		OS:           os,
		Architecture: arch,
		Variant:      variant,
	}, nil
}

func PlatformFromString(p string) (*ggcrv1.Platform, error) {
	plat, err := ggcrv1.ParsePlatform(p)
	if err != nil {
		return nil, err
	}
	if plat.OS != "linux" {
		return nil, fmt.Errorf("%q is not a valid platform OS for apptainer", plat.OS)
	}

	plat.Architecture, plat.Variant = normalizeArch(plat.Architecture, plat.Variant)

	return plat, nil
}

func PlatformFromArch(a string) (*ggcrv1.Platform, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("%q is not a valid platform OS for apptainer", runtime.GOOS)
	}

	arch, variant := normalizeArch(a, "")

	return &ggcrv1.Platform{
		OS:           runtime.GOOS,
		Architecture: arch,
		Variant:      variant,
	}, nil
}
