// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/image/driver"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

// OverlayMount represents an overlayfs mount used for build --overlay.
// It tracks the upper, work, and merged directories
type OverlayMount struct {
	UpperDir  string       // path to writable upper layer
	WorkDir   string       // overlayfs work directory
	MergedDir string       // path where overlayfs is mounted (the build rootfs)
	Driver    image.Driver // image driver for overlay mount (may be nil)
}

// SetupOverlayMount sets up overlayfs for a build, with the base image as a
// read-only lower layer and a writable upper layer for build changes.
// Returns an OverlayMount structure to track the directories for cleanup.
func SetupOverlayMount(basePath, tmpDir string) (*OverlayMount, error) {
	// Check if running as root (user id 0)
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	if currentUser.Uid != "0" {
		return nil, fmt.Errorf("'build --overlay' requires root or fakeroot privileges (current uid: %s)", currentUser.Uid)
	}

	// Create directories for overlay: upper, work, merged
	overlayParent, err := os.MkdirTemp(tmpDir, "overlay-")
	if err != nil {
		return nil, fmt.Errorf("failed to create overlay parent directory: %w", err)
	}

	upperDir := filepath.Join(overlayParent, "upper")
	workDir := filepath.Join(overlayParent, "work")
	mergedDir := filepath.Join(overlayParent, "merged")

	// Create directories
	for _, dir := range []string{upperDir, workDir, mergedDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			os.RemoveAll(overlayParent)
			return nil, fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Mount overlayfs via image driver instead of using kernel overlayfs,
	// because the latter can't do as much unprivileged.  In particular
	// it quickly runs into a need for the redirect_dir option, and that
	// is effectively a privileged option.
	appfile := apptainerconf.GetCurrentConfig()
	driver.InitImageDrivers(true, true, appfile, image.OverlayFeature)
	drv := image.GetDriver(driver.DriverName)

	if drv == nil {
		os.RemoveAll(overlayParent)
		return nil, fmt.Errorf("image driver not available for overlay mounts")
	}

	if drv.Features()&image.OverlayFeature == 0 {
		os.RemoveAll(overlayParent)
		return nil, fmt.Errorf("overlay feature not supported by image driver")
	}

	if err := drv.Start(nil, 0, false); err != nil {
		os.RemoveAll(overlayParent)
		return nil, fmt.Errorf("failed to start image driver: %w", err)
	}

	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", basePath, upperDir, workDir)
	mountParams := &image.MountParams{
		Target:     mergedDir,
		Filesystem: "overlay",
		FSOptions:  []string{options},
	}

	if err := drv.Mount(mountParams, nil); err != nil {
		drv.Stop(mergedDir)
		os.RemoveAll(overlayParent)
		return nil, fmt.Errorf("failed to mount overlay via image driver: %w", err)
	}

	return &OverlayMount{
		UpperDir:  upperDir,
		WorkDir:   workDir,
		MergedDir: mergedDir,
		Driver:    drv,
	}, nil
}

// TeardownOverlayMount unmounts the overlayfs
func TeardownOverlayMount(om *OverlayMount) error {
	if om == nil {
		return fmt.Errorf("overlay mount is nil")
	}

	if om.Driver == nil {
		return fmt.Errorf("internal error -- TeardownOverlayMount called with no image driver")
	}

	// Unmount the overlay and stop the image driver
	if err := syscall.Unmount(om.MergedDir, 0); err != nil {
		sylog.Infof("Error when umounting %v, skipping: %v", om.MergedDir, err)
	}
	if err := om.Driver.Stop(om.MergedDir); err != nil {
		return fmt.Errorf("failed to stop image driver: %w", err)
	}

	return nil
}

// hashBaseImage computes a sha256 hash of the base image file, used to tag
// an overlay-only build so its base can be found at runtime with
// `--basepath`/`APPTAINER_BASEPATH`.
func hashBaseImage(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("'build --overlay' does not support a sandbox base image %q; the base must be a SIF file", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
