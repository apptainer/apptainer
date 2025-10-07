// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createSquashfs creates a small but valid squashfs file that can be used
// with an image.
func createSquashfs(t *testing.T) string {
	sqshFilePath := filepath.Join(t.TempDir(), "test.sqfs")

	cmdBin, lookErr := exec.LookPath("mksquashfs")
	if lookErr != nil {
		t.Skipf("%s is not  available, skipping the test...", cmdBin)
	}

	cmd := exec.Command(cmdBin, t.TempDir(), sqshFilePath)
	cmdErr := cmd.Run()
	if cmdErr != nil {
		t.Fatalf("cannot create squashfs volume: %s\n", cmdErr)
	}

	return sqshFilePath
}

func TestCheckSquashfsHeader(t *testing.T) {
	sqshFilePath := createSquashfs(t)
	defer os.Remove(sqshFilePath)

	img, imgErr := os.Open(sqshFilePath)
	if imgErr != nil {
		t.Fatalf("cannot open file: %s\n", imgErr)
	}
	b := make([]byte, bufferSize)
	n, readErr := img.Read(b)
	if readErr != nil || n != bufferSize {
		t.Fatalf("cannot read the first %d bytes of the image file\n", bufferSize)
	}

	_, err := CheckSquashfsHeader(b)
	if err != nil {
		t.Fatalf("cannot check squashfs header of a valid image")
	}
}

func TestSquashfsInitializer(t *testing.T) {
	// Valid image test
	sqshFilePath := createSquashfs(t)
	defer os.Remove(sqshFilePath)

	var squashfsfmt squashfsFormat
	var err error
	mode := squashfsfmt.openMode(true)

	img := &Image{
		Path: sqshFilePath,
		Name: "test",
	}
	img.Writable = true
	img.File, err = os.OpenFile(sqshFilePath, mode, 0)
	if err != nil {
		t.Fatalf("cannot open image's file: %s\n", err)
	}
	fileinfo, err := img.File.Stat()
	if err != nil {
		img.File.Close()
		t.Fatalf("cannot stat the image file: %s\n", err)
	}

	// initializer must fail if writable is true
	err = squashfsfmt.initializer(img, fileinfo)
	if err == nil {
		t.Fatalf("unexpected success for squashfs initializer\n")
	}
	// reset cursor for header parsing
	img.File.Seek(0, io.SeekStart)
	// initialized must succeed if writable is false
	img.Writable = false
	err = squashfsfmt.initializer(img, fileinfo)
	if err != nil {
		t.Fatalf("unexpected error for squashfs initializer: %s\n", err)
	}
	img.File.Close()

	// Invalid image
	invalidPath := t.TempDir()
	img.File, err = os.Open(invalidPath)
	if err != nil {
		t.Fatalf("open() failed: %s\n", err)
	}
	defer img.File.Close()
	fileinfo, err = img.File.Stat()
	if err != nil {
		t.Fatalf("cannot stat file pointer: %s\n", err)
	}

	err = squashfsfmt.initializer(img, fileinfo)
	if err == nil {
		t.Fatal("squashfs succeeded with a directory while expected to fail")
	}
}

func TestSFSOpenMode(t *testing.T) {
	var squashfsfmt squashfsFormat

	// Yes, openMode() for squashfs always returns os.O_RDONLY
	if squashfsfmt.openMode(true) != os.O_RDONLY {
		t.Fatal("openMode(true) returned the wrong value")
	}
	if squashfsfmt.openMode(false) != os.O_RDONLY {
		t.Fatal("openMode(false) returned the wrong value")
	}
}
