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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func createTar(t *testing.T) string {
	tarFilePath := filepath.Join(t.TempDir(), "test.tar")

	dir := t.TempDir()
	testFile := filepath.Join(dir, "testfile")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("cannot create test file: %s", err)
	}

	cmd := exec.Command("tar", "-cf", tarFilePath, "-C", dir, "testfile")
	if err := cmd.Run(); err != nil {
		t.Fatalf("cannot create tar file: %s", err)
	}

	return tarFilePath
}

func TestCheckTarHeader(t *testing.T) {
	tarFilePath := createTar(t)
	defer os.Remove(tarFilePath)

	img, imgErr := os.Open(tarFilePath)
	if imgErr != nil {
		t.Fatalf("cannot open file: %s\n", imgErr)
	}
	b := make([]byte, tarBlockSize)
	n, readErr := img.Read(b)
	if readErr != nil || n != tarBlockSize {
		t.Fatalf("cannot read the first %d bytes of the tar file\n", tarBlockSize)
	}

	err := CheckTarHeader(b)
	if err != nil {
		t.Fatalf("cannot check tar header of a valid image: %s", err)
	}
}

func TestTarInitializer(t *testing.T) {
	tarFilePath := createTar(t)
	defer os.Remove(tarFilePath)

	var tarfmt tarFormat
	var err error
	mode := tarfmt.openMode(true)

	img := &Image{
		Path: tarFilePath,
		Name: "test",
	}
	img.Writable = true
	img.File, err = os.OpenFile(tarFilePath, mode, 0)
	if err != nil {
		t.Fatalf("cannot open image's file: %s\n", err)
	}
	fileinfo, err := img.File.Stat()
	if err != nil {
		img.File.Close()
		t.Fatalf("cannot stat the image file: %s\n", err)
	}

	err = tarfmt.initializer(img, fileinfo)
	if err != nil {
		t.Fatalf("unexpected error for tar initializer: %s\n", err)
	}
	img.File.Close()

	if img.Type != TAR {
		t.Fatalf("expected type TAR, got %d", img.Type)
	}

	if len(img.Partitions) != 1 {
		t.Fatalf("expected 1 partition, got %d", len(img.Partitions))
	}

	partition := img.Partitions[0]
	if partition.Type != TAR {
		t.Fatalf("expected partition type TAR, got %d", partition.Type)
	}

	if partition.AllowedUsage != DataUsage {
		t.Fatalf("expected partition usage DataUsage, got %d", partition.AllowedUsage)
	}
}

func TestTarOpenMode(t *testing.T) {
	var tarfmt tarFormat

	if tarfmt.openMode(true) != os.O_RDWR {
		t.Fatal("openMode(true) returned the wrong value")
	}
	if tarfmt.openMode(false) != os.O_RDONLY {
		t.Fatal("openMode(false) returned the wrong value")
	}
}

func TestTarInvalidHeader(t *testing.T) {
	invalidPath := t.TempDir() + "/invalid.tar"
	if err := os.WriteFile(invalidPath, []byte("not a tar file"), 0o644); err != nil {
		t.Fatalf("cannot create invalid tar file: %s", err)
	}
	defer os.Remove(invalidPath)

	img, err := os.Open(invalidPath)
	if err != nil {
		t.Fatalf("cannot open file: %s\n", err)
	}
	defer img.Close()

	b := make([]byte, tarBlockSize)
	n, readErr := img.Read(b)
	if readErr != nil || n != tarBlockSize {
		if n < tarBlockSize {
			err = CheckTarHeader(b[:n])
			if err == nil {
				t.Fatal("expected error for invalid tar header, got nil")
			}
			return
		}
		t.Fatalf("cannot read the first %d bytes of the file\n", tarBlockSize)
	}

	err = CheckTarHeader(b)
	if err == nil {
		t.Fatal("expected error for invalid tar header, got nil")
	}
}

func TestTarInitializerDirectory(t *testing.T) {
	dirPath := t.TempDir()

	var tarfmt tarFormat
	var err error
	mode := tarfmt.openMode(false)

	img := &Image{
		Path: dirPath,
		Name: "test",
	}
	img.Writable = false
	img.File, err = os.OpenFile(dirPath, mode, 0)
	if err != nil {
		t.Fatalf("cannot open directory: %s\n", err)
	}
	fileinfo, err := img.File.Stat()
	if err != nil {
		img.File.Close()
		t.Fatalf("cannot stat the directory: %s\n", err)
	}

	err = tarfmt.initializer(img, fileinfo)
	if err == nil {
		t.Fatal("tar initializer succeeded with a directory while expected to fail")
	}
}

func TestTarInitializerWithCompression(t *testing.T) {
	cases := []struct {
		name string
		ext  string
	}{
		{name: "gzip", ext: ".tar.gz"},
		{name: "xz", ext: ".tar.xz"},
		{name: "zst", ext: ".tar.zst"},
		{name: "lz4", ext: ".tar.lz4"},
		{name: "lzo", ext: ".tar.lzo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tarFilePath := filepath.Join(dir, "test"+tc.ext)

			// Create a simple tar file first
			simpleTar := filepath.Join(dir, "test.tar")
			testDir := t.TempDir()
			testFile := filepath.Join(testDir, "testfile")
			if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
				t.Fatalf("cannot create test file: %s", err)
			}

			cmd := exec.Command("tar", "-cf", simpleTar, "-C", testDir, "testfile")
			if err := cmd.Run(); err != nil {
				t.Fatalf("cannot create tar file: %s", err)
			}

			// Compress it
			var compressCmd string
			switch tc.ext {
			case ".tar.gz":
				compressCmd = "gzip"
			case ".tar.xz":
				compressCmd = "xz"
			case ".tar.zst":
				compressCmd = "zstd"
			case ".tar.lz4":
				compressCmd = "lz4"
			case ".tar.lzo":
				compressCmd = "lzop"
			}

			if _, err := exec.LookPath(compressCmd); err != nil {
				t.Skipf("skipping %s: %s not found", tc.ext, compressCmd)
			}

			cmd = exec.Command(compressCmd, "-c", simpleTar)
			outputFile, err := os.Create(tarFilePath)
			if err != nil {
				t.Fatalf("cannot create output file: %s", err)
			}
			defer outputFile.Close()
			cmd.Stdout = outputFile
			if err := cmd.Run(); err != nil {
				t.Fatalf("compression failed for %s: %s", tc.name, err)
			}

			// Test with the initializer
			var tarfmt tarFormat
			img := &Image{
				Path: tarFilePath,
				Name: "test",
			}
			img.Writable = false
			img.File, err = os.OpenFile(tarFilePath, os.O_RDONLY, 0)
			if err != nil {
				t.Fatalf("cannot open compressed tar file: %s\n", err)
			}
			defer img.File.Close()
			fileinfo, err := img.File.Stat()
			if err != nil {
				t.Fatalf("cannot stat the image file: %s\n", err)
			}

			err = tarfmt.initializer(img, fileinfo)
			if err != nil {
				t.Fatalf("unexpected error for compressed tar initializer: %s\n", err)
			}

			if img.Type != TAR {
				t.Fatalf("expected type TAR, got %d", img.Type)
			}
		})
	}
}
