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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/image/driver"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/inspect"
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

// hashBaseImageFromFile computes a sha256 hash of the base image file content.
func hashBaseImageFromFile(path string) (string, error) {
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

// getHashFromSIFLabels attempts to read the base digest hash from a SIF
// file's inspect-metadata.json labels. Returns empty string if any of
// 1. Can't open the file as a SIF
// 2. If there's an overlay partition in the SIF
// 3. If the SIF was built with a definition file
// 4. If the metadata does not include a hash
// 5. If the hash does not begin with "sha256"
func getHashFromSIFLabels(path string) (string, error) {
	img, err := image.Init(path, false, false)
	if err != nil {
		sylog.Debugf("Can't get hash from SIF labels because %v", err)
		return "", nil
	}
	defer img.File.Close()

	partitions, err := img.GetOverlayPartitions()
	if err != nil {
		sylog.Debugf("Failed getting partition info because %v", err)
		return "", nil
	}
	for _, partition := range partitions {
		if partition.Type == image.EXT3 {
			return "", fmt.Errorf("overlay base image includes an ext3 overlay partition")
		}
	}
	if len(partitions) > 0 {
		sylog.Debugf("Skipping using hash from SIF labels because there's an overlay partition")
		return "", nil
	}

	r, err := image.NewSectionReader(img, image.SIFDescInspectMetadataJSON, -1)
	if err != nil {
		sylog.Debugf("Failed getting inspect metadata because %v", err)
		return "", nil
	}

	metadata := new(inspect.Metadata)
	if err := json.NewDecoder(r).Decode(metadata); err != nil {
		sylog.Debugf("Couldn't decode inspect-metadata.json: %v", err)
		return "", nil
	}

	// Check for "%" in the definition file because if no
	// definition file is used Apptainer inserts a small default
	// one with only "bootstrap" and "from".
	if strings.Contains(metadata.Attributes.Deffile, "%") {
		sylog.Debugf("Skipping using base digest stored in sif because deffile was used")
		return "", nil
	}

	if digest, ok := metadata.Attributes.Labels["org.opencontainers.image.base.digest"]; ok {
		if strings.HasPrefix(digest, "sha256:") {
			return digest, nil
		}
		sylog.Debugf("Skipping using base digest stored in sif because it did not begin with 'sha256:'")
	}
	sylog.Debugf("Base digest not found in sif's inspect-metadata.json")

	return "", nil
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

	if hash, err := getHashFromSIFLabels(path); err != nil {
		return "", err
	} else if hash != "" {
		sylog.Debugf("Using base digest %s from SIF labels", hash)
		return hash, nil
	}

	return hashBaseImageFromFile(path)
}
