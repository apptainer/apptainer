// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/sys/user"
)

// CopyWithTar is a wrapper around the docker pkg/archive/copy CopyWithTar
// function allowing unprivileged use. It forces ownership to the current
// uid/gid in unprivileged situations. No file may be copied into a location
// above dst, and hard links / symlinks may not target a file above dst.
func CopyWithTar(src, dst string) error {
	ar := archive.NewDefaultArchiver()

	// If we are running unprivileged, then squash uid / gid as necessary.
	// TODO: In future, we want to think about preserving effective ownership
	// for fakeroot cases where there will be a mapping allowing non-root, non-user
	// ownership to be preserved.
	euid := os.Geteuid()
	egid := os.Getgid()
	if euid != 0 || egid != 0 {
		sylog.Debugf("Using unprivileged CopyWithTar (uid=%d, gid=%d)", euid, egid)
		// The docker CopytWithTar function assumes it should create the top-level of dst as the
		// container root user. If we are unprivileged this means setting up an ID mapping
		// from UID/GID 0 to our host UID/GID.
		ar.IDMapping = user.IdentityMapping{
			// Single entry mapping of container root (0) to current uid only
			UIDMaps: []user.IDMap{
				{
					ID:       0,
					ParentID: int64(euid),
					Count:    1,
				},
			},
			// Single entry mapping of container root (0) to current gid only
			GIDMaps: []user.IDMap{
				{
					ID:       0,
					ParentID: int64(egid),
					Count:    1,
				},
			},
		}
		// Actual extraction of files needs to be *always* squashed to our current uid & gid.
		// This requires clearing the IDMaps, and setting a forced UID/GID with ChownOpts for
		// the lower level Untar func called by the archiver.
		chownOpts := &archive.ChownOpts{
			UID: euid,
			GID: egid,
		}
		ar.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
			options.IDMap = user.IdentityMapping{}
			options.ChownOpts = chownOpts
			return archive.Untar(tarArchive, dest, options)
		}
	}

	return ar.CopyWithTar(src, dst)
}

// CopyWithTarWithRoots copies files as with CopyWithTar, but uses a custom
// ar.Untar which checks that hard link / symlink targets are under dstRoot,
// and not dst. This allows copying a directory src to dst, where the directory
// contains a link to a location above dst, but under dstroot.
func CopyWithTarWithRoot(src, dst, dstRoot string) error {
	ar := archive.NewDefaultArchiver()

	// If we are running unprivileged, then squash uid / gid as necessary.
	// TODO: In future, we want to think about preserving effective ownership
	// for fakeroot cases where there will be a mapping allowing non-root, non-user
	// ownership to be preserved.
	euid := os.Geteuid()
	egid := os.Getgid()
	var chownOpts *archive.ChownOpts

	if euid != 0 || egid != 0 {
		sylog.Debugf("Using unprivileged CopyWithTar (uid=%d, gid=%d)", euid, egid)
		// The docker CopytWithTar function assumes it should create the top-level of dst as the
		// container root user. If we are unprivileged this means setting up an ID mapping
		// from UID/GID 0 to our host UID/GID.
		ar.IDMapping = user.IdentityMapping{
			// Single entry mapping of container root (0) to current uid only
			UIDMaps: []user.IDMap{
				{
					ID:       0,
					ParentID: int64(euid),
					Count:    1,
				},
			},
			// Single entry mapping of container root (0) to current gid only
			GIDMaps: []user.IDMap{
				{
					ID:       0,
					ParentID: int64(egid),
					Count:    1,
				},
			},
		}
		// Actual extraction of files needs to be *always* squashed to our current uid & gid.
		// This requires clearing the IDMaps, and setting a forced UID/GID with ChownOpts for
		// the lower level Untar func called by the archiver.
		chownOpts = &archive.ChownOpts{
			UID: euid,
			GID: egid,
		}
	}

	ar.Untar = func(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
		if options == nil {
			options = &archive.TarOptions{}
		}

		if chownOpts != nil {
			options.IDMap = user.IdentityMapping{}
			options.ChownOpts = chownOpts
		}

		if tarArchive == nil {
			return fmt.Errorf("empty archive")
		}
		dest = filepath.Clean(dest)
		if options.ExcludePatterns == nil {
			options.ExcludePatterns = []string{}
		}

		decompressedArchive, err := compression.DecompressStream(tarArchive)
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()

		return UnpackWithRoot(decompressedArchive, dest, dstRoot, options)
	}

	return ar.CopyWithTar(src, dst)
}
