// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.
//
// The following code is adapted from:
//
//	https://github.com/google/go-containerregistry/blob/v0.15.2/pkg/authn/keychain.go
//
// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/cryptkey"
	keyClient "github.com/apptainer/container-key-client/client"
	ocitypes "github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"golang.org/x/sys/unix"
)

const OCIConfigJSON = "oci-config"

// Bundle is the temporary environment used during the image building process.
type Bundle struct {
	JSONObjects map[string][]byte `json:"jsonObjects"`
	Recipe      Definition        `json:"rawDeffile"`
	Opts        Options           `json:"opts"`

	RootfsPath string `json:"rootfsPath"` // where actual fs to chroot will appear
	TmpDir     string `json:"tmpPath"`    // where temp files required during build will appear

	parentPath string // parent directory for RootfsPath
}

// Options defines build time behavior to be executed on the bundle.
type Options struct {
	// Sections are the parts of the definition to run during the build.
	Sections []string `json:"sections"`
	// TmpDir specifies a non-standard temporary location to perform a build.
	TmpDir string
	// LibraryURL contains URL to library where base images can be pulled.
	LibraryURL string `json:"libraryURL"`
	// LibraryAuthToken contains authentication token to access specified library.
	LibraryAuthToken string `json:"libraryAuthToken"`
	// Path to fakeroot command will be empty if not needed or not available
	FakerootPath string `json:"fakerootPath"`
	// KeyServerOpts contains options for keyserver used for SIF fingerprint verification in builds.
	KeyServerOpts []keyClient.Option
	// If non-nil, provides credentials to be used when authenticating to OCI registries.
	OCIAuthConfig *authn.AuthConfig
	// If non-nil, provides credentials to be used when authenticating to OCI registries.
	// Deprecated: Use OCIAuthConfig, which takes precedence if both are set.
	DockerAuthConfig *ocitypes.DockerAuthConfig
	// Custom docker Daemon host
	DockerDaemonHost string
	// EncryptionKeyInfo specifies the key used for filesystem
	// encryption if applicable.
	// A nil value indicates encryption should not occur.
	EncryptionKeyInfo *cryptkey.KeyInfo
	// ImgCache stores a pointer to the image cache to use.
	ImgCache *cache.Handle
	// NoTest indicates if build should skip running the test script.
	NoTest bool `json:"noTest"`
	// Force automatically deletes an existing container at build destination while performing build.
	Force bool `json:"force"`
	// Update detects and builds using an existing sandbox container at build destination.
	Update bool `json:"update"`
	// NoHTTPS instructs builder not to use secure connection.
	NoHTTPS bool `json:"noHTTPS"`
	// NoCleanUp allows a user to prevent a bundle from being cleaned up after a failed build.
	// useful for debugging.
	NoCleanUp bool `json:"noCleanUp"`
	// NoCache when true, will not use any cache, or make cache.
	NoCache bool
	// FixPerms controls if we will ensure owner rwX on container content
	// to preserve <=3.4 behavior.
	// TODO: Deprecate in 3.6, remove in 3.8
	FixPerms bool
	// To warn when the above is needed, we need to know if the target of this
	// bundle will be a sandbox
	SandboxTarget bool
	// Binds stores bind mounts used for the post scripts
	Binds []string
	// whether using gocryptfs to build and run encrypted containers
	Unprivilege bool
	// Arch info
	Arch string
	// Authentication file for registry credentials
	ReqAuthFile string
	// Which Platform to use when retrieving images for the build
	Platform ggcrv1.Platform
}

// NewEncryptedBundle creates an Encrypted Bundle environment.
func NewEncryptedBundle(parentPath, tempDir string, keyInfo *cryptkey.KeyInfo) (b *Bundle, err error) {
	return newBundle(parentPath, tempDir, keyInfo)
}

// NewBundle creates a Bundle environment.
func NewBundle(parentPath, tempDir string) (b *Bundle, err error) {
	return newBundle(parentPath, tempDir, nil)
}

// RunSection iterates through the sections specified in a bundle
// and returns true if the given string, s, is a section of the
// definition that should be executed during the build process.
func (b *Bundle) RunSection(s string) bool {
	for _, section := range b.Opts.Sections {
		if section == "none" {
			return false
		}
		if section == "all" || section == s {
			return true
		}
	}
	return false
}

// Remove cleans up any bundle files.
func (b *Bundle) Remove() error {
	var errors []string
	for _, dir := range []string{b.TmpDir, b.parentPath} {
		if err := fs.ForceRemoveAll(dir); err != nil {
			errors = append(errors, fmt.Sprintf("could not remove %q: %v", dir, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, " "))
	}
	return nil
}

func canChown(rootfs string) (bool, error) {
	// we always return true when building as user otherwise
	// build process would always fail at this step
	if os.Getuid() != 0 {
		return true, nil
	}

	chownFile := filepath.Join(rootfs, ".chownTest")

	f, err := os.OpenFile(chownFile, os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return false, fmt.Errorf("could not create %q: %v", chownFile, err)
	}
	defer f.Close()
	defer os.Remove(chownFile)

	if err := f.Chown(1, 1); os.IsPermission(err) {
		return false, nil
	}

	return true, nil
}

func cleanupDir(path string) {
	if err := os.Remove(path); err != nil {
		sylog.Errorf("Could not cleanup dir %q: %v", path, err)
	}
}

// newBundle creates a minimum bundle with root filesystem in parentPath.
// Any temporary files created during build process will be in tempDir/bundle-temp-*
// directory, that will be cleaned up after successful build.
//
// TODO: much of the logic in this func should likely be re-factored to func newBuild in the
// internal/pkg/build package, since it is the sole caller and has conditional logic which depends
// on implementation details of this package. In particular, chown() handling should be done at the
// build level, rather than the bundle level, to avoid repetition during multi-stage builds, and
// clarify responsibility for cleanup of the various directories that are created during the build
// process.
func newBundle(parentPath, tempDir string, keyInfo *cryptkey.KeyInfo) (*Bundle, error) {
	rootfsPath := filepath.Join(parentPath, "rootfs")

	tmpPath, err := os.MkdirTemp(tempDir, "bundle-temp-")
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir in %q: %v", tempDir, err)
	}
	sylog.Debugf("Created temporary directory %q for the bundle", tmpPath)

	if err := os.MkdirAll(rootfsPath, 0o755); err != nil {
		cleanupDir(tmpPath)
		cleanupDir(parentPath)
		return nil, fmt.Errorf("could not create %q: %v", rootfsPath, err)
	}

	// check that chown works with the underlying filesystem containing
	// the temporary sandbox image
	can, err := canChown(rootfsPath)
	if err != nil {
		cleanupDir(tmpPath)
		cleanupDir(rootfsPath)
		cleanupDir(parentPath)
		return nil, err
	} else if !can {
		cleanupDir(rootfsPath)
		cleanupDir(parentPath)

		// If the supplied rootfs was not inside tempDir (as is the case during a sandbox build),
		// try tempDir as a fallback.
		if !strings.HasPrefix(parentPath, tempDir) {
			parentPath, err = os.MkdirTemp(tempDir, "build-temp-")
			if err != nil {
				cleanupDir(tmpPath)
				return nil, fmt.Errorf("failed to create rootfs directory: %v", err)
			}
			// Create an inner dir, so we don't clobber the secure permissions on the surrounding dir.
			rootfsNewPath := filepath.Join(parentPath, "rootfs")
			if err := os.Mkdir(rootfsNewPath, 0o755); err != nil {
				cleanupDir(tmpPath)
				cleanupDir(parentPath)
				return nil, fmt.Errorf("could not create rootfs dir in %q: %v", rootfsNewPath, err)
			}
			// check that chown works with the underlying filesystem pointed
			// by $TMPDIR and return an error if chown doesn't work
			can, err := canChown(rootfsNewPath)
			if err != nil {
				cleanupDir(tmpPath)
				cleanupDir(rootfsNewPath)
				cleanupDir(parentPath)
				return nil, err
			} else if !can {
				cleanupDir(tmpPath)
				cleanupDir(rootfsNewPath)
				cleanupDir(parentPath)
				sylog.Errorf("Could not set files/directories ownership, if %s is on a network filesystem, "+
					"you must set TMPDIR to a local path (eg: TMPDIR=/var/tmp apptainer build ...)", rootfsNewPath)
				return nil, fmt.Errorf("ownership change not allowed in %s, aborting", tempDir)
			}
			rootfsPath = rootfsNewPath
		}
	}

	sylog.Debugf("Created directory %q for the bundle", rootfsPath)

	return &Bundle{
		parentPath:  parentPath,
		RootfsPath:  rootfsPath,
		TmpDir:      tmpPath,
		JSONObjects: make(map[string][]byte),
		Opts: Options{
			EncryptionKeyInfo: keyInfo,
		},
	}, nil
}

// FixPerms will work through the rootfs of this bundle, making sure that all
// files and directories have permissions set such that the owner can read,
// modify, delete. This brings us to the situation of <=3.4
func FixPerms(rootfs string) (err error) {
	errors := 0
	err = fs.PermWalk(rootfs, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			sylog.Errorf("Unable to access rootfs path %s: %s", path, err)
			errors++
			return nil
		}

		switch mode := f.Mode(); {
		// Directories must have the owner 'rx' bits to allow traversal and reading on move, and the 'w' bit
		// so their content can be deleted by the user when the rootfs/sandbox is deleted
		case mode.IsDir():
			if err := os.Chmod(path, f.Mode().Perm()|0o700); err != nil {
				sylog.Errorf("Error setting permission for %s: %s", path, err)
				errors++
			}
		case mode.IsRegular():
			// Regular files must have the owner 'r' bit so that everything can be read in order to
			// copy or move the rootfs/sandbox around. Also, the `w` bit as the build does write into
			// some files (e.g. resolv.conf) in the container rootfs.
			if err := os.Chmod(path, f.Mode().Perm()|0o600); err != nil {
				sylog.Errorf("Error setting permission for %s: %s", path, err)
				errors++
			}
		}
		return nil
	})

	if errors > 0 {
		err = fmt.Errorf("%d errors were encountered when setting permissions", errors)
	}
	return err
}
