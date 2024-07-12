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

	"github.com/sylabs/singularity/v4/internal/pkg/build/sources"
	"github.com/sylabs/singularity/v4/internal/pkg/image/packer"
	"github.com/sylabs/singularity/v4/internal/pkg/util/fs"
	"github.com/sylabs/singularity/v4/pkg/build/types"
)

func createArchiveFromDir(dir string, t *testing.T) *os.File {
	mk, err := exec.LookPath("mksquashfs")
	if err != nil {
		t.SkipNow()
	}
	f, err := os.CreateTemp("", "archive-")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(mk, dir, f.Name(), "-noappend", "-no-progress")
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

func TestSquashfsInput(t *testing.T) {
	if s := packer.NewSquashfs(); !s.HasMksquashfs() {
		t.Skip("mksquashfs not found, skipping")
	}

	dir, err := os.MkdirTemp(os.TempDir(), "test-localpacker-squashfs-")
	if err != nil {
		t.Fatalf("while creating tmpdir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Check that folder symlinks in the image to common folders (in base env) work
	inputDir := filepath.Join(dir, "input")

	makeDir(filepath.Join(dir, "var", "tmp"), t)
	// Symlink /var/tmp -> /tmp in image
	makeDir(filepath.Join(inputDir, "tmp"), t)
	makeDir(filepath.Join(inputDir, "var"), t)
	if err := os.Symlink("../tmp", filepath.Join(inputDir, "var", "tmp")); err != nil {
		t.Fatalf("while creating symlink: %v", err)
	}
	// And a file we can check for
	testfile := "conveyorPacker_local_test.go"
	data, err := os.ReadFile(testfile)
	if err != nil {
		t.Fatalf("while reading test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, testfile), data, 0o644); err != nil {
		t.Fatalf("while writing test file: %v", err)
	}
	archive := createArchiveFromDir(inputDir, t)
	defer os.Remove(archive.Name())

	bundleTmp, _ := os.MkdirTemp(os.TempDir(), "bundle-tmp-")
	b, err := types.NewBundle(dir, bundleTmp)
	if err != nil {
		t.Fatalf("while creating bundle: %v", err)
	}
	b.Recipe, _ = types.NewDefinitionFromURI("localimage://" + archive.Name())

	lcp := &sources.LocalConveyorPacker{}
	if err := lcp.Get(context.Background(), b); err != nil {
		t.Fatalf("while getting local packer: %v", err)
	}

	_, err = lcp.Pack(context.Background())
	if err != nil {
		t.Fatalf("failed to Pack from %s: %v\n", archive.Name(), err)
	}
	rootfs := b.RootfsPath

	// check if testfile was extracted
	path := filepath.Join(rootfs, testfile)
	if !isExist(path) {
		t.Errorf("extraction failed, %s is missing", path)
	}
	// Check folders and symlinks
	path = filepath.Join(rootfs, "tmp")
	if !fs.IsDir(path) {
		t.Errorf("extraction failed, %s is missing", path)
	}
	path = filepath.Join(rootfs, "var")
	if !fs.IsDir(path) {
		t.Errorf("extraction failed, %s is missing", path)
	}
	path = filepath.Join(rootfs, "var", "tmp")
	if !isExist(path) {
		t.Errorf("extraction failed, %s is missing", path)
	} else if !fs.IsLink(path) {
		t.Errorf("extraction failed, %s is not a symlink", path)
	} else {
		tgt, _ := os.Readlink(path)
		if tgt != "../tmp" {
			t.Errorf("extraction failed, %s wrongly points to %s", path, tgt)
		}
	}
}
