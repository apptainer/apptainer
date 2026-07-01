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
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
	"github.com/blang/semver/v4"
)

// hasWorkingPtrace returns true if the ptrace() system call is usable.
// proot relies on ptrace, which can be unavailable because of a seccomp
// filter, an AppArmor/Yama restriction, or because Apptainer itself is
// running inside another container without the CAP_SYS_PTRACE capability.
func hasWorkingPtrace() bool {
	// Use the currently running executable as the traced child: since
	// PTRACE_TRACEME causes the child to stop with SIGTRAP right after
	// the exec call and before running any of its own code, it doesn't
	// matter which binary is exec'd as long as it exists and is runnable.
	cmd := exec.Command("/proc/self/exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{Ptrace: true}
	err := cmd.Start()
	if err == nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return err == nil
}

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
				minVer := semver.MustParse("4.6.0")
				hasPercentage = sv.GTE(minVer)
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

	prog := s.MksquashfsPath
	extraEnv := []string{}
	if namespaces.IsUnprivileged() {
		// building as unprivileged user, make the files appear as root
		ignoreProot := os.Getenv("APPTAINER_IGNORE_PROOT")
		proot, err := bin.FindBin("proot")
		if ignoreProot == "" && err == nil {
			if hasWorkingPtrace() {
				// Insert proot around mksquashfs to take advantage of
				// file owner and group information stored by umoci in
				// a "rootlesscontainers" extended attribute.
				// https://github.com/apptainer/apptainer/issues/2830
				args = append([]string{"-S", "/", prog}, args...)
				prog = proot
				// Add the MALLOC settings to workaround segfaults seen
				// in mksquashfs on Ubuntu 22.04
				// https://github.com/apptainer/apptainer/issues/3486
				// https://github.com/apptainer/apptainer/issues/3560
				extraEnv = append(extraEnv,
					"MALLOC_MMAP_MAX_=0",
					"MALLOC_ARENA_MAX=1000000",
				)
			} else {
				sylog.Infof("Skipping preservation of file ownership because ptrace() does not work")
			}
		}
		if prog != proot {
			args = append(args, "-all-root")
		}
	}

	// mksquashfs -reproducible automatically clamps everything to SOURCE_DATE_EPOCH
	// (note: -reproducible is the default, there is also a -not-reproducible option)
	sylog.Verbosef("Executing %s %s", prog, strings.Join(args, " "))
	cmd := exec.Command(prog, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
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
