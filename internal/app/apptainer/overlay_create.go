// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/fakeroot"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/ccoveille/go-safecast"
	"golang.org/x/sys/unix"
)

const (
	mkfsBinary     = "mkfs.ext3"
	ddBinary       = "dd"
	truncateBinary = "truncate"
)

// isSigned returns true if the SIF in rw contains one or more signature objects.
func isSigned(rw sif.ReadWriter) (bool, error) {
	f, err := sif.LoadContainer(rw,
		sif.OptLoadWithFlag(os.O_RDONLY),
		sif.OptLoadWithCloseOnUnload(false),
	)
	if err != nil {
		return false, err
	}
	defer f.UnloadContainer()

	sigs, err := f.GetDescriptors(sif.WithDataType(sif.DataSignature))
	return len(sigs) > 0, err
}

// addOverlayToImage adds the EXT3 overlay at overlayPath to the SIF image at imagePath.
func addOverlayToImage(imagePath, overlayPath string) error {
	f, err := sif.LoadContainerFromPath(imagePath)
	if err != nil {
		return err
	}
	defer f.UnloadContainer()

	tf, err := os.Open(overlayPath)
	if err != nil {
		return err
	}
	defer tf.Close()

	arch := f.PrimaryArch()
	if arch == "unknown" {
		arch = runtime.GOARCH
	}

	di, err := sif.NewDescriptorInput(sif.DataPartition, tf,
		sif.OptPartitionMetadata(sif.FsExt3, sif.PartOverlay, arch),
	)
	if err != nil {
		return err
	}

	return f.AddObject(di)
}

// findConvertCommand finds dd unless overlaySparse is true
func findConvertCommand(overlaySparse bool) (string, error) {
	// We can support additional arguments, so return a list
	command := ""

	// Sparse overlay requires truncate -s
	if overlaySparse {
		truncate, err := bin.FindBin(truncateBinary)
		if err != nil {
			return command, err
		}
		command = truncate

		// Regular (non sparse) requires dd
	} else {
		dd, err := bin.FindBin(ddBinary)
		if err != nil {
			return command, err
		}
		command = dd
	}
	return command, nil
}

// OverlayCreate creates the overlay with an optional size, image path, dirs, fakeroot and sparse option.
//
//nolint:maintidx
func OverlayCreate(size int, imgPath string, tmpDir string, overlaySparse bool, isFakeroot bool, overlayDirs ...string) error {
	if size < 64 {
		return fmt.Errorf("image size must be equal or greater than 64 MiB")
	}

	mkfs, err := bin.FindBin(mkfsBinary)
	if err != nil {
		return err
	}

	// This can be dd or truncate (if supported and --sparse is true)
	convertCommand, err := findConvertCommand(overlaySparse)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)

	// check if -d option is available
	cmd := exec.Command(mkfs, "--help")
	cmd.Stderr = buf
	// ignore error because the command always returns with exit code 1
	_ = cmd.Run()

	if !strings.Contains(buf.String(), "[-d ") {
		return fmt.Errorf("%s seems too old as it doesn't support -d, this is required to create the overlay layout", mkfsBinary)
	}

	sifImage := false

	if err := unix.Access(imgPath, unix.W_OK); err == nil {
		img, err := image.Init(imgPath, false)
		if err != nil {
			return fmt.Errorf("while opening image file %s: %s", imgPath, err)
		}
		switch img.Type {
		case image.SIF:
			sysPart, err := img.GetRootFsPartition()
			if err != nil {
				return fmt.Errorf("while getting root FS partition: %s", err)
			} else if sysPart.Type == image.ENCRYPTSQUASHFS {
				return fmt.Errorf("encrypted root FS partition in %s: could not add writable overlay", imgPath)
			}

			overlays, err := img.GetOverlayPartitions()
			if err != nil {
				return fmt.Errorf("while getting SIF overlay partitions: %s", err)
			}
			signed, err := isSigned(img.File)
			if err != nil {
				return fmt.Errorf("while getting SIF info: %s", err)
			} else if signed {
				return fmt.Errorf("SIF image %s is signed: could not add writable overlay", imgPath)
			}

			img.File.Close()

			for _, overlay := range overlays {
				if overlay.Type != image.EXT3 {
					continue
				}
				delCmd := fmt.Sprintf("apptainer sif del %d %s", overlay.ID, imgPath)
				return fmt.Errorf("a writable overlay partition already exists in %s (ID: %d), delete it first with %q", imgPath, overlay.ID, delCmd)
			}

			sifImage = true
		case image.EXT3:
			return fmt.Errorf("EXT3 overlay image %s already exists", imgPath)
		default:
			return fmt.Errorf("destination image must be SIF image")
		}
	}

	tmpFile := imgPath + ".ext3"
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	errBuf := new(bytes.Buffer)

	// truncate has a different interaction than dd
	if strings.Contains(convertCommand, "truncate") {
		cmd = exec.Command(convertCommand, fmt.Sprintf("--size=%dM", size), tmpFile)
	} else {
		cmd = exec.Command(convertCommand, "if=/dev/zero", "of="+tmpFile, "bs=1M", fmt.Sprintf("count=%d", size))
	}

	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while zero'ing overlay image %s: %s\nCommand error: %s", tmpFile, err, errBuf)
	}
	errBuf.Reset()

	if err := os.Chmod(tmpFile, 0o600); err != nil {
		return fmt.Errorf("while setting 0600 permission on %s: %s", tmpFile, err)
	}

	tmpDir, err = os.MkdirTemp(tmpDir, "overlay-")
	if err != nil {
		return fmt.Errorf("while creating temporary overlay directory: %s", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	perm := os.FileMode(0o755)

	uid := os.Getuid()
	if uid > 65535 || os.Getgid() > 65535 {
		perm = 0o777
	}

	upperDir := filepath.Join(tmpDir, "upper")
	workDir := filepath.Join(tmpDir, "work")

	oldumask := unix.Umask(0)
	defer unix.Umask(oldumask)

	if uid != 0 {
		uid32, err := safecast.Convert[uint32](uid)
		if err != nil {
			return fmt.Errorf("failed to convert UID to uint32: %s", err)
		}
		if !fakeroot.IsUIDMapped(uid32) {
			// Using --fakeroot here for use with --fakeroot
			//  overlay is only necessary when only root-mapped
			//  user namespaces are in use: real root of course
			//  can override, and the kernel allows fake root to
			//  override the user's ownership if the user's id is
			//  mapped via /etc/subuid.  In the case of using only
			//  the fakeroot command (in suid flow with no user
			//  namespaces), using the --fakeroot option here
			//  prevents overlay from working, most unfortunately.
			if !fakeroot.UserNamespaceAvailable() {
				if isFakeroot {
					sylog.Infof("User namespaces are not available, so using --fakeroot here would")
					sylog.Infof("  actually interfere with fakeroot command overlay operation")
					return fmt.Errorf("--fakeroot used without user namespaces")
				}
			} else if !isFakeroot {
				sylog.Infof("Creating overlay image for use without fakeroot.")
				sylog.Infof("Consider re-running with --fakeroot option.")
			}
		}

		if isFakeroot {
			sylog.Debugf("Trying root-mapped namespace")
			err = fakeroot.UnshareRootMapped(os.Args, false)
			if err == nil {
				// everything was done by the child
				os.Exit(0)
			}
			return fmt.Errorf("failed to start fakeroot: %v", err)
		}
	}

	if err := os.Mkdir(upperDir, perm); err != nil {
		return fmt.Errorf("while creating %s: %s", upperDir, err)
	}
	if err := os.Mkdir(workDir, perm); err != nil {
		return fmt.Errorf("while creating %s: %s", workDir, err)
	}

	for _, dir := range overlayDirs {
		od := filepath.Join(upperDir, dir)
		if !strings.HasPrefix(od, upperDir) {
			return fmt.Errorf("overlay directory created outside of overlay layout %s", upperDir)
		}
		if err := os.MkdirAll(od, perm); err != nil {
			return fmt.Errorf("while creating %s: %s", od, err)
		}
	}

	cmd = exec.Command(mkfs, "-d", tmpDir, tmpFile)
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while creating ext3 partition in %s: %s\nCommand error: %s", tmpFile, err, errBuf)
	}
	errBuf.Reset()

	if sifImage {
		if err := addOverlayToImage(imgPath, tmpFile); err != nil {
			return fmt.Errorf("while adding ext3 overlay partition to %s: %w", imgPath, err)
		}
	} else {
		if err := os.Rename(tmpFile, imgPath); err != nil {
			return fmt.Errorf("while renaming %s to %s: %s", tmpFile, imgPath, err)
		}
	}

	return nil
}
