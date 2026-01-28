// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/build/sources"
	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/pkg/build/types"
)

const (
	shubURI = "shub://ikaneshiro/singularityhub:latest"
)

// TestShubConveyor tests if we can pull an image from singularity hub
func TestShubConveyor(t *testing.T) {
	// TODO(mem): re-enable this; disabled while shub is down
	t.Skip("Skipping tests that access singularity hub")

	if testing.Short() {
		t.SkipNow()
	}

	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-shub"), os.TempDir())
	if err != nil {
		return
	}

	b.Recipe, err = types.NewDefinitionFromURI(shubURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", shubURI, err)
	}

	cp := &sources.ShubConveyorPacker{}

	err = cp.Get(t.Context(), b)
	// clean up tmpfs since assembler isn't called
	defer cp.CleanUp()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", shubURI, err)
	}
}

// TestShubPacker checks if we can create a Bundle from the pulled image
func TestShubPacker(t *testing.T) {
	// TODO(mem): re-enable this; disabled while shub is down
	t.Skip("Skipping tests that access singularity hub")
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-shub"), os.TempDir())
	if err != nil {
		return
	}

	b.Recipe, err = types.NewDefinitionFromURI(shubURI)
	if err != nil {
		t.Fatalf("unable to parse URI %s: %v\n", shubURI, err)
	}

	scp := &sources.ShubConveyorPacker{}

	err = scp.Get(t.Context(), b)
	// clean up tmpfs since assembler isn't called
	defer scp.CleanUp()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", shubURI, err)
	}

	_, err = scp.Pack(t.Context())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", shubURI, err)
	}
}
