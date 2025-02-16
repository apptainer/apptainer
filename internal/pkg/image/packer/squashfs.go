// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package packer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/blang/semver/v4"
)

// Squashfs represents a squashfs packer
type Squashfs struct {
	MksquashfsPath string
}

// NewSquashfs initializes and returns a Squashfs packer instance
func NewSquashfs() *Squashfs {
	s := &Squashfs{}
	s.MksquashfsPath, _ = bin.FindBin("mksquashfs")
	return s
}

// HasMksquashfs returns if mksquashfs binary has set or not
func (s Squashfs) HasMksquashfs() bool {
	return s.MksquashfsPath != ""
}

func (s Squashfs) create(files []string, dest string, opts []string) error {
	var stderr bytes.Buffer

	if !s.HasMksquashfs() {
		return fmt.Errorf("could not create squashfs, mksquashfs not found")
	}

	// check if mksquashfs is new enough to have the -percentage option
	hasPercentage := false
	if out, err := exec.Command(s.MksquashfsPath, "-version").Output(); err == nil {
		line := strings.Split(string(out), "\n")[0]
		if strings.HasPrefix(line, "mksquashfs version ") {
			v := strings.Split(line, " ")[2]
			if sv, err := semver.ParseTolerant(v); err == nil {
				sylog.Debugf("mksquashfs version: %s", sv)
				min := semver.MustParse("4.6.0")
				hasPercentage = sv.GTE(min)
			}
		}
	}

	// mksquashfs takes args of the form: source1 source2 ... destination [options]
	args := files
	args = append(args, dest)
	args = append(args, opts...)

	progressBar := &client.PercentageProgressBar{}
	defer progressBar.Abort(true)
	if sylog.GetLevel() < int(sylog.VerboseLevel) && hasPercentage {
		args = append(args, "-quiet", "-percentage")
	}

	sylog.Verbosef("Executing %s %s", s.MksquashfsPath, strings.Join(args, " "))
	cmd := exec.Command(s.MksquashfsPath, args...)
	if sylog.GetLevel() >= int(sylog.VerboseLevel) {
		cmd.Stdout = os.Stdout
	} else if hasPercentage {
		progressBar.Init()
		cmd.Stdout = progressBar.GetWriter()
	} else {
		sylog.Infof("To see mksquashfs output with progress bar enable verbose logging")
	}
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s command failed: %v: %s", s.MksquashfsPath, err, stderr.String())
	}
	progressBar.Done()
	progressBar.Wait()
	return nil
}

// Create makes a squashfs filesystem from a list of source files/directories to a
// destination file
func (s Squashfs) Create(src []string, dest string, opts []string) error {
	return s.create(src, dest, opts)
}
