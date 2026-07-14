// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/build/types"
)

func testBundle(rootfsPath string) (*types.Bundle, error) {
	rootfs, err := os.OpenRoot(rootfsPath)
	if err != nil {
		return nil, err
	}
	return &types.Bundle{RootfsPath: rootfsPath, Rootfs: rootfs}, nil
}

func testWithGoodBundle(t *testing.T, f func(b *types.Bundle) error) {
	b, err := testBundle(t.TempDir())
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}
	defer b.Rootfs.Close()

	if err := f(b); err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}
}

func testWithBadBundle(t *testing.T, f func(b *types.Bundle) error) {
	b, err := testBundle("/does/not/exist")
	if err == nil {
		defer b.Rootfs.Close()
		err = f(b)
	}
	if err == nil {
		t.Fatalf("Unexpected success with bad directory")
	}
}

func TestMakeDirs(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	testWithGoodBundle(t, makeDirs)
	testWithBadBundle(t, makeDirs)
}

func TestMakeSymlinks(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	testWithGoodBundle(t, makeSymlinks)
	testWithBadBundle(t, makeSymlinks)
	testWithGoodBundle(t, func(b *types.Bundle) error {
		// makeSymlinks should be idempotent
		if err := makeSymlinks(b); err != nil {
			return err
		}
		return makeSymlinks(b)
	})
}

func TestMakeSymlink(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	const oldname = ".singularity.d/runscript"
	const newname = "singularity"

	tests := []struct {
		name  string
		setup func(root *os.Root) error // setup the precondition at newname

		wantSymlinkTo string // expected target if a symlink is expected
		wantFile      bool   // expect a regular file (not a symlink) left in place
	}{
		{
			name:          "Missing",
			setup:         func(*os.Root) error { return nil },
			wantSymlinkTo: oldname,
		},
		{
			name:          "CorrectSymlink",
			setup:         func(r *os.Root) error { return r.Symlink(oldname, newname) },
			wantSymlinkTo: oldname,
		},
		{
			name:          "WrongSymlink",
			setup:         func(r *os.Root) error { return r.Symlink("/.singularity.d/wrong_link", newname) },
			wantSymlinkTo: oldname,
		},
		{
			name:     "RegularFile",
			setup:    func(r *os.Root) error { return r.WriteFile(newname, []byte("legacy"), 0o644) },
			wantFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := testBundle(t.TempDir())
			if err != nil {
				t.Fatalf("testBundle: %v", err)
			}
			defer b.Rootfs.Close()

			if err := tt.setup(b.Rootfs); err != nil {
				t.Fatalf("setup: %v", err)
			}

			if err := makeSymlink(b.Rootfs, oldname, newname); err != nil {
				t.Fatalf("makeSymlink: %v", err)
			}

			full := filepath.Join(b.RootfsPath, newname)
			fi, err := os.Lstat(full)
			if err != nil {
				t.Fatalf("lstat %s: %v", full, err)
			}

			if tt.wantFile {
				if fi.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("expected regular file, got a symlink")
				}
				return
			}

			if fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("expected symlink, got mode %v", fi.Mode())
			}
			target, err := os.Readlink(full)
			if err != nil {
				t.Fatalf("readlink: %v", err)
			}
			if target != tt.wantSymlinkTo {
				t.Fatalf("symlink target = %q, want %q", target, tt.wantSymlinkTo)
			}
		})
	}
}

func TestMakeFiles(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	testWithGoodBundle(t, func(b *types.Bundle) error {
		if err := makeDirs(b); err != nil {
			return err
		}
		return makeFiles(b, false)
	})
	testWithBadBundle(t, func(b *types.Bundle) error { return makeFiles(b, false) })
	// #4532 - Check that we can succeed with an existing file that doesn't have
	// write permission.
	testWithGoodBundle(t, func(b *types.Bundle) error {
		if err := makeDirs(b); err != nil {
			return err
		}
		err := fs.EnsureFileWithPermission(filepath.Join(b.RootfsPath, "etc", "hosts"), 0o400)
		if err != nil {
			t.Fatalf("Failed to make test hosts file: %s", err)
		}
		return makeFiles(b, false)
	})
}

func TestMakeBaseEnv(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	testWithGoodBundle(t, func(b *types.Bundle) error { return makeBaseEnv(b, false) })
	testWithBadBundle(t, func(b *types.Bundle) error { return makeBaseEnv(b, false) })
}
