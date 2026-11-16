// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2024, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"
	"runtime"

	"github.com/apptainer/apptainer/internal/pkg/client/oras"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// OrasConveyorPacker only needs to hold a packer to pack the image it pulls
// as well as extra information about the library it's pulling from.
type OrasConveyorPacker struct {
	LocalPacker
}

// Get downloads container from Apptainerhub
func (cp *OrasConveyorPacker) Get(ctx context.Context, b *types.Bundle) (err error) {
	sylog.Debugf("Getting container from registry using ORAS")

	// uri with leading // for oras handlers to consume
	ref := "//" + b.Recipe.Header["from"]
	// full uri for name determination and output
	fullRef := "oras:" + ref

	imagePath, digest, err := oras.PullWithDigest(ctx, b.Opts.ImgCache, fullRef, runtime.GOARCH, b.Opts.TmpDir, b.Opts.OCIAuthConfig, b.Opts.NoHTTPS, b.Opts.ReqAuthFile)
	if err != nil {
		return fmt.Errorf("while fetching library image: %v", err)
	}

	// Record the base image identity (the org.opencontainers.image.base.name
	// and base.digest labels; b.Opts.Digest also doubles as the overlay
	// base-hash for basepath lookup).
	//
	// If the pulled SIF already records both of these (e.g. it was built from
	// a docker:// base and then pushed), inherit them so the original
	// provenance is preserved: the SIF is the same bytes wherever it is
	// stored, so its recorded base points at the same image.
	//
	// Otherwise derive the name from the oras:// reference used to pull and
	// use the pulled image's manifest digest. Note: the manifest digest is
	// the least-stable choice for a base-hash (it changes on each push); the
	// SIF content checksum would be more stable under reproducible builds and
	// is a candidate follow-up, but for now the OCI digest is used to stay
	// consistent with the docker:// path.
	//
	// Either way, both b.Opts.Tag and b.Opts.Digest end up set.
	repo, tag, inputDigest := ociimage.SplitTagAndDigest(b.Recipe.Header["from"])
	if existingName, existingDigest, err := oras.ImageBase(imagePath); err == nil && existingName != "" && existingDigest != "" {
		sylog.Debugf("ORAS pulled SIF %q already records base.name=%q base.digest=%q; inheriting them", imagePath, existingName, existingDigest)
		b.Opts.Tag = existingName
		b.Opts.Digest = existingDigest
	} else {
		if err != nil {
			sylog.Debugf("Could not read base labels from ORAS pulled SIF %q (%v); using URI-derived values", imagePath, err)
		}
		baseName, baseErr := oras.BaseName(repo, tag, inputDigest)
		if baseErr != nil {
			return fmt.Errorf("while deriving base.name for %q: %w", b.Recipe.Header["from"], baseErr)
		}
		b.Opts.Tag = baseName
		b.Opts.Digest = digest.String()
	}

	// insert base metadata before unpacking fs
	if err = makeBaseEnv(b, true); err != nil {
		return fmt.Errorf("while inserting base environment: %v", err)
	}

	cp.LocalPacker, err = GetLocalPacker(ctx, imagePath, b)
	return err
}
