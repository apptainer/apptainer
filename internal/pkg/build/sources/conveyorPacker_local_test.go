// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2024, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/build/sources"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/build/types"
)

func createArchiveFromDir(dir string, t *testing.T) *os.File {
	require.Command(t, "mksquashfs")
	f, err := os.CreateTemp("", "archive-")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("mksquashfs", dir, f.Name(), "-noappend", "-no-progress")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return f
}

func makeDir(path string, t *testing.T) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("while creating directory %s: %v", path, err)
	}
}

func isExist(path string) bool {
	result, _ := fs.PathExists(path)
	return result
}

func TestLocalPackerSquashfs(t *testing.T) {
	require.Command(t, "mksquashfs")

	tempDirPath, err := os.MkdirTemp("", "test-localpacker-squashfs")
	if err != nil {
		t.Fatalf("while creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDirPath)

	// Image root directory
	inputDir := filepath.Join(tempDirPath, "input")

	// Create directories
	makeDir(filepath.Join(inputDir, "tmp"), t)
	makeDir(filepath.Join(inputDir, "var"), t)

	// Create symlinks: /var/tmp -> /tmp , /var/log -> /tmp
	if err := os.Symlink("/tmp", filepath.Join(inputDir, "var", "tmp")); err != nil {
		t.Fatalf("while creating symlink: %v", err)
	}
	if err := os.Symlink("/tmp", filepath.Join(inputDir, "var", "log")); err != nil {
		t.Fatalf("while creating symlink: %v", err)
	}

	// Add a file we can check for and create image.
	testfile := "conveyorPacker_local_test.go"
	data, err := os.ReadFile(testfile)
	if err != nil {
		t.Fatalf("while reading test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, testfile), data, 0o644); err != nil {
		t.Fatalf("while writing test file: %v", err)
	}
	image := createArchiveFromDir(inputDir, t).Name()
	defer os.Remove(image)

	// Create bundle
	bundleTmp, _ := os.MkdirTemp("", "bundle-tmp-")
	defer os.RemoveAll(bundleTmp)

	b, err := types.NewBundle(tempDirPath, bundleTmp)
	if err != nil {
		t.Fatalf("while creating bundle: %v", err)
	}
	b.Recipe, _ = types.NewDefinitionFromURI("localimage://" + image)

	// Create and execute packer
	lcp := &sources.LocalConveyorPacker{}
	if err := lcp.Get(context.Background(), b); err != nil {
		t.Fatalf("while getting local packer: %v", err)
	}
	if _, err = lcp.Pack(context.Background()); err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", image, err)
	}
	rootfsPath := b.RootfsPath

	// Check if testfile was extracted
	path := filepath.Join(rootfsPath, testfile)
	if !isExist(path) {
		t.Errorf("extraction failed, %s is missing", path)
	}

	// Check directories
	path = filepath.Join(rootfsPath, "tmp")
	if !fs.IsDir(path) {
		t.Errorf("extraction failed, %s is missing", path)
	}
	path = filepath.Join(rootfsPath, "var")
	if !fs.IsDir(path) {
		t.Errorf("extraction failed, %s is missing", path)
	}

	// Check symlinks
	// /var/tmp -> /tmp
	path = filepath.Join(rootfsPath, "var", "tmp")
	if !isExist(path) {
		t.Errorf("extraction failed, %s is missing", path)
	} else if !fs.IsLink(path) {
		t.Errorf("extraction failed, %s is not a symlink", path)
	} else {
		tgt, _ := os.Readlink(path)
		if tgt != "/tmp" {
			t.Errorf("extraction failed, %s wrongly points to %s", path, tgt)
		}
	}
	// /var/log -> /tmp
	path = filepath.Join(rootfsPath, "var", "log")
	if !isExist(path) {
		t.Errorf("extraction failed, %s is missing", path)
	} else if !fs.IsLink(path) {
		t.Errorf("extraction failed, %s is not a symlink", path)
	} else {
		tgt, _ := os.Readlink(path)
		if tgt != "/tmp" {
			t.Errorf("extraction failed, %s wrongly points to %s", path, tgt)
		}
	}
}
