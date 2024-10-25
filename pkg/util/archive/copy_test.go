// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package archive

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
)

func TestCopyWithTar(t *testing.T) {
	copyFunc := func(src, dst string) error {
		return CopyWithTar(src, dst)
	}

	t.Run("privileged", func(t *testing.T) {
		test.EnsurePrivilege(t)
		testCopy(t, copyFunc)
	})

	t.Run("unprivileged", func(t *testing.T) {
		test.DropPrivilege(t)
		defer test.ResetPrivilege(t)
		testCopy(t, copyFunc)
	})
}

func TestCopyWithTarRoot(t *testing.T) {
	copyFunc := func(src, dst string) error {
		return CopyWithTarWithRoot(src, dst, dst)
	}

	t.Run("privileged", func(t *testing.T) {
		test.EnsurePrivilege(t)
		testCopy(t, copyFunc)
	})

	t.Run("unprivileged", func(t *testing.T) {
		test.DropPrivilege(t)
		defer test.ResetPrivilege(t)
		testCopy(t, copyFunc)
	})

	test.DropPrivilege(t)
	t.Run("relLinkTarget", testRelLinkTarget)
}

func testCopy(t *testing.T, copyFunc func(src, dst string) error) {
	srcRoot := t.TempDir()
	t.Logf("srcRoot location: %s\n", srcRoot)

	// Source Files
	srcFile := filepath.Join(srcRoot, "srcFile")
	if err := os.WriteFile(srcFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Source Dirs
	srcDir := filepath.Join(srcRoot, "srcDir")
	if err := os.Mkdir(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Source Symlink
	srcLink := filepath.Join(srcRoot, "srcLink")
	if err := os.Symlink("srcFile", srcLink); err != nil {
		t.Fatal(err)
	}

	dstRoot := t.TempDir()
	t.Logf("dstRoot location: %s\n", dstRoot)

	// Perform the actual copy to a subdir of our dst tempdir.
	// This ensures CopyWithTar has to create the dest directory, which is
	// where the non-wrapped call would fail for unprivileged users.
	err := copyFunc(srcRoot, path.Join(dstRoot, "dst"))
	if err != nil {
		t.Fatalf("Error during CopyWithTar: %v", err)
	}

	tests := []struct {
		name       string
		expectPath string
		expectFile bool
		expectDir  bool
		expectLink bool
	}{
		{
			name:       "file",
			expectPath: "dst/srcFile",
			expectFile: true,
		},
		{
			name:       "dir",
			expectPath: "dst/srcDir",
			expectDir:  true,
		},
		{
			name:       "symlink",
			expectPath: "dst/srcLink",
			expectFile: true,
			expectLink: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dstFinal := filepath.Join(dstRoot, tt.expectPath)
			// verify file was copied
			_, err = os.Stat(dstFinal)
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("while checking for destination file: %s", err)
			}
			if os.IsNotExist(err) {
				t.Errorf("expected destination %s does not exist", dstFinal)
			}

			// File when expected?
			if tt.expectFile && !fs.IsFile(dstFinal) {
				t.Errorf("destination %s should be a file, but isn't", dstFinal)
			}
			// Dir when expected?
			if tt.expectDir && !fs.IsDir(dstFinal) {
				t.Errorf("destination %s should be a directory, but isn't", dstFinal)
			}
			// Symlink when expected
			if tt.expectLink && !fs.IsLink(dstFinal) {
				t.Errorf("destination %s should be a symlink, but isn't", dstFinal)
			}
			if !tt.expectLink && fs.IsLink(dstFinal) {
				t.Errorf("destination %s should be a symlink, but is", dstFinal)
			}
		})
	}
}

// Test that CopyWithTarWithRoot doesn't allow relative symlink targets above
// the dstRoot, but does allow them within the dstRoot, above dst.
//
// See - https://github.com/sylabs/singularity/issues/2607
func testRelLinkTarget(t *testing.T) {
	tmpDir := t.TempDir()

	linkTargetFile := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(linkTargetFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		linkPath    string
		linkTarget  string
		src         string
		dst         string
		dstRoot     string
		expectError bool
	}{
		{
			name:        "symlinkEscape",
			linkPath:    filepath.Join(tmpDir, "symlinkEscape", "myLink"),
			linkTarget:  "../target",
			src:         filepath.Join(tmpDir, "symlinkEscape"),
			dst:         filepath.Join(tmpDir, "symlinkEscapeDest"),
			dstRoot:     filepath.Join(tmpDir, "symlinkEscapeDest"),
			expectError: true,
		},
		{
			name:        "symLinkWithinRoot",
			linkPath:    filepath.Join(tmpDir, "symlinkWithinRoot", "myLink"),
			linkTarget:  "../target",
			src:         filepath.Join(tmpDir, "symlinkWithinRoot"),
			dst:         filepath.Join(tmpDir, "symlinkWithinRootDest"),
			dstRoot:     tmpDir,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Dir(tt.linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tt.linkTarget, tt.linkPath); err != nil {
				t.Fatal(err)
			}
			err := CopyWithTarWithRoot(tt.src, tt.dst, tt.dstRoot)
			if err == nil && tt.expectError {
				sylog.Errorf("expected error, but none returned")
			}
			if err != nil && !tt.expectError {
				sylog.Errorf("unexpected error %v", err)
			}
		})
	}
}
