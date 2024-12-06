// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/build/sources"
	"github.com/apptainer/apptainer/internal/pkg/ociplatform"
	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/pkg/build/types"
)

const (
	libraryURL = "https://library.production.sycloud.io/"
	libraryURI = "library://alpine:latest"
)

// TestLibraryConveyor tests if we can pull an image from singularity hub
func TestLibraryConveyor(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	test.EnsurePrivilege(t)

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-library"), os.TempDir())
	if err != nil {
		return
	}

	b.Opts.LibraryURL = libraryURL
	p, err := ociplatform.DefaultPlatform()
	if err != nil {
		t.Fatalf("failed to get DefaultPlatform: %v", err)
	}
	b.Opts.Platform = *p

	b.Recipe, err = types.NewDefinitionFromURI(libraryURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", libraryURI, err)
	}

	cp := &sources.LibraryConveyorPacker{}

	// set a clean image cache
	imgCache, cleanup := setupCache(t)
	defer cleanup()
	b.Opts.ImgCache = imgCache

	err = cp.Get(context.Background(), b)
	// clean up tmpfs since assembler isn't called
	defer cp.CleanUp()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", libraryURI, err)
	}
}

// TestLibraryPacker checks if we can create a Bundle from the pulled image
func TestLibraryPacker(t *testing.T) {
	test.EnsurePrivilege(t)

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-library"), os.TempDir())
	if err != nil {
		return
	}

	b.Opts.LibraryURL = libraryURL

	p, err := ociplatform.DefaultPlatform()
	if err != nil {
		t.Fatalf("failed to get DefaultPlatform: %v", err)
	}
	b.Opts.Platform = *p

	b.Recipe, err = types.NewDefinitionFromURI(libraryURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", libraryURI, err)
	}

	cp := &sources.LibraryConveyorPacker{}

	// set a clean image cache
	imgCache, cleanup := setupCache(t)
	defer cleanup()
	b.Opts.ImgCache = imgCache

	err = cp.Get(context.Background(), b)
	// clean up tmpfs since assembler isn't called
	defer cp.CleanUp()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", libraryURI, err)
	}

	_, err = cp.Pack(context.Background())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", libraryURI, err)
	}
}
