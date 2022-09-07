// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package unpacker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
	"golang.org/x/sys/unix"
)

const (
	stdinFile = "/proc/self/fd/0"

	// exclude 'dev/' directory from extraction for non root users
	excludeDevRegex = `^(.{0}[^d]|.{1}[^e]|.{2}[^v]|.{3}[^\x2f]).*$`
)

var cmdFunc func(unsquashfs string, dest string, filename string, filter string, opts ...string) (*exec.Cmd, error)

// unsquashfsCmd is the command instance for executing unsquashfs command
// in a non sandboxed environment when this package is used for unit tests.
func unsquashfsCmd(unsquashfs string, dest string, filename string, filter string, opts ...string) (*exec.Cmd, error) {
	args := []string{}
	args = append(args, opts...)
	// remove the destination directory if any, if the directory is
	// not empty (typically during image build), the unsafe option -f is
	// set, this is unfortunately required by image build
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to remove %s: %s", dest, err)
		}
		// unsafe mode
		args = append(args, "-f")
	}

	args = append(args, "-d", dest, filename)
	if filter != "" {
		args = append(args, filter)
	}

	sylog.Debugf("Calling %s %v", unsquashfs, args)
	return exec.Command(unsquashfs, args...), nil
}

// Squashfs represents a squashfs unpacker.
type Squashfs struct {
	UnsquashfsPath string
}

// NewSquashfs initializes and returns a Squahfs unpacker instance
func NewSquashfs() *Squashfs {
	s := &Squashfs{}
	s.UnsquashfsPath, _ = bin.FindBin("unsquashfs")
	return s
}

// HasUnsquashfs returns if unsquashfs binary has been found or not
func (s *Squashfs) HasUnsquashfs() bool {
	return s.UnsquashfsPath != ""
}

func (s *Squashfs) extract(files []string, reader io.Reader, dest string) (err error) {
	if !s.HasUnsquashfs() {
		return fmt.Errorf("could not extract squashfs data, unsquashfs not found")
	}

	// pipe over stdin by default
	stdin := true
	filename := stdinFile

	if _, ok := reader.(*os.File); !ok {
		// use the destination parent directory to store the
		// temporary archive
		tmpdir := filepath.Dir(dest)

		// unsquashfs doesn't support to send file content over
		// a stdin pipe since it use lseek for every read it does
		tmp, err := os.CreateTemp(tmpdir, "archive-")
		if err != nil {
			return fmt.Errorf("failed to create staging file: %s", err)
		}
		filename = tmp.Name()
		stdin = false
		defer os.Remove(filename)

		if _, err := io.Copy(tmp, reader); err != nil {
			return fmt.Errorf("failed to copy content in staging file: %s", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("failed to close staging file: %s", err)
		}
	}

	// First we try unsquashfs with appropriate xattr options. If we are in
	// rootless mode we need "-user-xattrs" so we don't try to set system xattrs
	// that require root. However we must check (user) xattrs are supported on
	// the FS as unsquashfs >=4.4 will give a non-zero error code if it cannot
	// set them, e.g. on tmpfs (#5668)
	opts := []string{}
	hostuid, err := namespaces.HostUID()
	if err != nil {
		return fmt.Errorf("could not get host UID: %s", err)
	}
	rootless := hostuid != 0

	// Does our target filesystem support user xattrs?
	ok, err := TestUserXattr(filepath.Dir(dest))
	if err != nil {
		return err
	}
	// If we are in rootless mode & we support user xattrs, set -user-xattrs so that user xattrs are extracted, but
	// system xattrs are ignored (needs root).
	if ok && rootless {
		opts = append(opts, "-user-xattrs")
	}
	// If user-xattrs aren't supported we need to disable setting of all xattrs.
	if !ok {
		opts = append(opts, "-no-xattrs")
	}

	// non real root users could not create pseudo devices so we compare
	// the host UID (to include fake root user) and apply a filter at extraction (#5690)
	filter := ""

	// exclude dev directory only if there no specific files provided for extraction
	// as globbing won't work with POSIX regex enabled
	if rootless && len(files) == 0 {
		sylog.Debugf("Excluding /dev directory during root filesystem extraction (non root user)")
		// filter requires POSIX regex
		opts = append(opts, "-r")
		filter = excludeDevRegex
	}
	defer func() {
		if err != nil || filter == "" {
			return
		}
		// create $rootfs/dev as it has been excluded
		rootfsDev := filepath.Join(dest, "dev")
		devErr := os.Mkdir(rootfsDev, 0o755)
		if devErr != nil && !os.IsExist(devErr) {
			err = fmt.Errorf("could not create %s: %s", rootfsDev, devErr)
		}
	}()

	// Now run unsquashfs with our 'best' options
	sylog.Debugf("Trying unsquashfs options: %v", opts)
	cmd, err := cmdFunc(s.UnsquashfsPath, dest, filename, filter, opts...)
	if err != nil {
		return fmt.Errorf("command error: %s", err)
	}
	cmd.Args = append(cmd.Args, files...)
	if stdin {
		cmd.Stdin = reader
	}

	o, err := cmd.CombinedOutput()

	sylog.Debugf("*** BEGIN WRAPPED UNSQUASHFS OUTPUT ***")
	sylog.Debugf(string(o))
	sylog.Debugf("*** END WRAPPED UNSQUASHFS OUTPUT ***")

	if err != nil {
		return fmt.Errorf("extract command failed: %s: %s", string(o), err)
	}

	return nil
}

// ExtractAll extracts a squashfs filesystem read from reader to a
// destination directory.
func (s *Squashfs) ExtractAll(reader io.Reader, dest string) error {
	return s.extract(nil, reader, dest)
}

// ExtractFiles extracts provided files from a squashfs filesystem
// read from reader to a destination directory.
func (s *Squashfs) ExtractFiles(files []string, reader io.Reader, dest string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to extract")
	}
	return s.extract(files, reader, dest)
}

// TestUserXattr tries to set a user xattr on PATH to ensure they are supported on this fs
func TestUserXattr(path string) (ok bool, err error) {
	tmp, err := os.CreateTemp(path, "uxattr-")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	tmp.Close()
	err = unix.Setxattr(tmp.Name(), "user.apptainer", []byte{}, 0)
	if err == unix.ENOTSUP || err == unix.EOPNOTSUPP {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("while testing user xattr support at %s: %v", tmp.Name(), err)
	}
	return true, nil
}
