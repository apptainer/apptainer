// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/build/types/parser"
)

const yumDef = "../../../../examples/centos/Apptainer"

func TestYumConveyor(t *testing.T) {
	// TODO - Centos puts non-amd64 at a different mirror location
	// need multiple def files to test on other archs
	require.Arch(t, "amd64")
	require.RPMMacro(t, "_db_backend", "bdb")
	require.RPMMacro(t, "_dbpath", "/var/lib/rpm")

	if testing.Short() {
		t.SkipNow()
	}

	_, dnfErr := bin.FindBin("dnf")
	_, yumErr := bin.FindBin("yum")
	if dnfErr != nil && yumErr != nil {
		t.Skip("skipping test, neither dnf nor yum found")
	}

	test.EnsurePrivilege(t)

	defFile, err := os.Open(yumDef)
	if err != nil {
		t.Fatalf("unable to open file %s: %v\n", yumDef, err)
	}
	defer defFile.Close()

	// create bundle to build into
	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-yum"), os.TempDir())
	if err != nil {
		return
	}

	b.Recipe, err = parser.ParseDefinitionFile(defFile)
	if err != nil {
		t.Fatalf("failed to parse definition file %s: %v\n", yumDef, err)
	}

	yc := &YumConveyor{}

	err = yc.Get(context.Background(), b)
	// clean up bundle since assembler isn't called
	defer yc.b.Remove()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", yumDef, err)
	}
}

func TestYumPacker(t *testing.T) {
	// TODO - Centos puts non-amd64 at a different mirror location
	// need multiple def files to test on other archs
	require.Arch(t, "amd64")
	require.RPMMacro(t, "_db_backend", "bdb")
	require.RPMMacro(t, "_dbpath", "/var/lib/rpm")

	if testing.Short() {
		t.SkipNow()
	}

	_, dnfErr := exec.LookPath("dnf")
	_, yumErr := exec.LookPath("yum")
	if dnfErr != nil && yumErr != nil {
		t.Skip("skipping test, neither dnf nor yum found")
	}

	test.EnsurePrivilege(t)

	defFile, err := os.Open(yumDef)
	if err != nil {
		t.Fatalf("unable to open file %s: %v\n", yumDef, err)
	}
	defer defFile.Close()

	// create bundle to build into
	b, err := types.NewBundle(filepath.Join(os.TempDir(), "sbuild-yum"), os.TempDir())
	if err != nil {
		return
	}

	b.Recipe, err = parser.ParseDefinitionFile(defFile)
	if err != nil {
		t.Fatalf("failed to parse definition file %s: %v\n", yumDef, err)
	}

	ycp := &YumConveyorPacker{}

	err = ycp.Get(context.Background(), b)
	// clean up tmpfs since assembler isn't called
	defer ycp.b.Remove()
	if err != nil {
		t.Fatalf("failed to Get from %s: %v\n", yumDef, err)
	}

	_, err = ycp.Pack(context.Background())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", yumDef, err)
	}
}
