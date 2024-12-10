// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// TODO(ian): The build package should be refactored to make each conveyorpacker
// its own separate package. With that change, this file should be grouped with the
// OCIConveyorPacker code

package sources

import (
	"context"
	"errors"
	"fmt"
	"os"

	apexlog "github.com/apex/log"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	umocilayer "github.com/opencontainers/umoci/oci/layer"
	"github.com/opencontainers/umoci/pkg/idtools"
)

// isExtractable checks if we have extractable layers in the image. Shouldn't be
// an ORAS artifact or similar. If we don't check, ggcr mutate.Extract will
// happily create an empty rootfs, leading to odd error messages elsewhere.
func isExtractable(img v1.Image) (bool, error) {
	layers, err := img.Layers()
	if err != nil {
		return false, err
	}
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return false, err
		}
		if mt.IsLayer() {
			return true, nil
		}
	}
	return false, nil
}

// UnpackRootfs extracts all of the layers of the given srcImage into destDir.
func UnpackRootfs(_ context.Context, srcImage v1.Image, destDir string) (err error) {
	extractable, err := isExtractable(srcImage)
	if err != nil {
		return err
	}
	if !extractable {
		return fmt.Errorf("no extractable OCI/Docker tar layers found in this image")
	}

	flatTar := mutate.Extract(srcImage)

	var mapOptions umocilayer.MapOptions

	loggerLevel := sylog.GetLevel()

	// set the apex log level, for umoci
	if loggerLevel <= int(sylog.ErrorLevel) {
		// silent option
		apexlog.SetLevel(apexlog.ErrorLevel)
	} else if loggerLevel <= int(sylog.LogLevel) {
		// quiet option
		apexlog.SetLevel(apexlog.WarnLevel)
	} else if loggerLevel < int(sylog.DebugLevel) {
		// verbose option(s) or default
		apexlog.SetLevel(apexlog.InfoLevel)
	} else {
		// debug option
		apexlog.SetLevel(apexlog.DebugLevel)
	}

	// Allow unpacking as non-root
	if namespaces.IsUnprivileged() {
		sylog.Debugf("setting umoci rootless mode")
		mapOptions.Rootless = true

		uidMap, err := idtools.ParseMapping(fmt.Sprintf("0:%d:1", os.Geteuid()))
		if err != nil {
			return fmt.Errorf("error parsing uidmap: %s", err)
		}
		mapOptions.UIDMappings = append(mapOptions.UIDMappings, uidMap)

		gidMap, err := idtools.ParseMapping(fmt.Sprintf("0:%d:1", os.Getegid()))
		if err != nil {
			return fmt.Errorf("error parsing gidmap: %s", err)
		}
		mapOptions.GIDMappings = append(mapOptions.GIDMappings, gidMap)
	}

	// Unpack root filesystem
	unpackOptions := umocilayer.UnpackOptions{MapOptions: mapOptions}
	err = umocilayer.UnpackLayer(destDir, flatTar, &unpackOptions)
	if err != nil {
		return fmt.Errorf("error unpacking rootfs: %s", err)
	}

	// No `--fix-perms` and no sandbox... we are fine
	return err
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

// CheckPerms will work through the rootfs of this bundle, and find if any
// directory does not have owner rwX - which may cause unexpected issues for a
// user trying to look through, or delete a sandbox
func CheckPerms(rootfs string) (err error) {
	// This is a locally defined error we can bubble up to cancel our recursive
	// structure.
	errRestrictivePerm := errors.New("restrictive file permission found")

	err = fs.PermWalkRaiseError(rootfs, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			// If the walk function cannot access a directory at all, that's an
			// obvious restrictive permission we need to warn on
			if os.IsPermission(err) {
				sylog.Debugf("Path %q has restrictive permissions", path)
				return errRestrictivePerm
			}
			return fmt.Errorf("unable to access rootfs path %s: %s", path, err)
		}
		// Warn on any directory not `rwX` - technically other combinations may
		// be traversable / removable
		if f.Mode().IsDir() && f.Mode().Perm()&0o700 != 0o700 {
			sylog.Debugf("Path %q has restrictive permissions", path)
			return errRestrictivePerm
		}
		return nil
	})

	if errors.Is(err, errRestrictivePerm) {
		sylog.Warningf("The sandbox contain files/dirs that cannot be removed with 'rm'.")
		sylog.Warningf("Use 'chmod -R u+rwX' to set permissions that allow removal.")
		sylog.Warningf("Use the '--fix-perms' option to 'apptainer build' to modify permissions at build time.")
		// It's not an error any further up... the rootfs is still usable
		return nil
	}
	return err
}
