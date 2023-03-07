// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package archive

import (
	"io"
	"os"

	"github.com/apptainer/apptainer/pkg/sylog"
	da "github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
)

// CopyWithTar is a wrapper around the docker pkg/archive/copy CopyWithTar allowing unprivileged use.
// It forces ownership to the current uid/gid in unprivileged situations.
//
// Disable context check as it raises a warning through the docker dependency we
// cannot modify to pass a context.
//
// nolint:contextcheck
func CopyWithTar(src, dst string) error {
	ar := da.NewDefaultArchiver()

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
		ar.IDMapping = idtools.IdentityMapping{
			// Single entry mapping of container root (0) to current uid only
			UIDMaps: []idtools.IDMap{
				{
					ContainerID: 0,
					HostID:      euid,
					Size:        1,
				},
			},
			// Single entry mapping of container root (0) to current gid only
			GIDMaps: []idtools.IDMap{
				{
					ContainerID: 0,
					HostID:      egid,
					Size:        1,
				},
			},
		}
		// Actual extraction of files needs to be *always* squashed to our current uid & gid.
		// This requires clearing the IDMaps, and setting a forced UID/GID with ChownOpts for
		// the lower level Untar func called by the archiver.
		eIdentity := &idtools.Identity{
			UID: euid,
			GID: egid,
		}
		ar.Untar = func(tarArchive io.Reader, dest string, options *da.TarOptions) error {
			options.IDMap = idtools.IdentityMapping{}
			options.ChownOpts = eIdentity
			return da.Untar(tarArchive, dest, options)
		}
	}

	return ar.CopyWithTar(src, dst)
}
