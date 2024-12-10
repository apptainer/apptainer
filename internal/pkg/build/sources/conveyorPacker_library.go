// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2024, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"
	"runtime"

	golog "github.com/go-log/log"

	"github.com/apptainer/apptainer/internal/pkg/client/library"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/container-library-client/client"
)

// LibraryConveyorPacker only needs to hold a packer to pack the image it pulls
// as well as extra information about the library it's pulling from
type LibraryConveyorPacker struct {
	b *types.Bundle
	LocalPacker
}

// Get downloads container from Sylabs Cloud Library.
func (cp *LibraryConveyorPacker) Get(ctx context.Context, b *types.Bundle) (err error) {
	sylog.Debugf("Getting container from Library")

	if b.Opts.ImgCache == nil {
		return fmt.Errorf("invalid image cache")
	}

	cp.b = b

	libraryURL := b.Opts.LibraryURL
	authToken := b.Opts.LibraryAuthToken

	// check for custom library from definition
	customLib, ok := b.Recipe.Header["library"]
	if ok {
		sylog.Debugf("Using custom library: %v", customLib)
		libraryURL = customLib
	}

	imageRef, err := library.NormalizeLibraryRef(b.Recipe.Header["from"])
	if err != nil {
		return fmt.Errorf("error parsing libraryRef: %v", err)
	}

	if imageRef.Host != "" {
		if b.Opts.NoHTTPS {
			libraryURL = "http://" + imageRef.Host
		} else {
			libraryURL = "https://" + imageRef.Host
		}
	}

	sylog.Debugf("LibraryURL: %v", libraryURL)
	sylog.Debugf("LibraryRef: %v", imageRef.String())

	pullOpts := library.PullOptions{
		LibraryConfig: &client.Config{
			BaseURL:   libraryURL,
			AuthToken: authToken,
			Logger:    (golog.Logger)(sylog.DebugLogger{}),
		},
	}
	imagePath, err := library.Pull(ctx, b.Opts.ImgCache, imageRef, runtime.GOARCH, cp.b.TmpDir, pullOpts)
	if err != nil {
		return fmt.Errorf("while fetching library image: %v", err)
	}

	// insert base metadata before unpacking fs
	if err = makeBaseEnv(cp.b.RootfsPath, true); err != nil {
		return fmt.Errorf("while inserting base environment: %v", err)
	}

	cp.LocalPacker, err = GetLocalPacker(ctx, imagePath, cp.b)

	return err
}

// CleanUp removes any files owned by the conveyorPacker on the filesystem.
func (cp *LibraryConveyorPacker) CleanUp() {
	cp.b.Remove()
}
