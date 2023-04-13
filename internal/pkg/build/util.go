// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package build

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/slice"
	"golang.org/x/sys/unix"
)

// destSubpath is the path to mount the file to the container.
// If destName is set to empty string (""), the source will be used.
func createStageFile(source string, destSubpath string, b *types.Bundle, warnMsg string) (string, error) {

	if destSubpath == "" {
		destSubpath = source
	}

	dest := filepath.Join(b.RootfsPath, destSubpath)
	if err := unix.Access(dest, unix.R_OK); err != nil {
		sylog.Warningf("%s: while accessing to %s: %s", warnMsg, dest, err)
		return "", nil
	}

	content, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %s", source, err)
	}

	// Append an extra blank line to the end of the staged file. This is a trick to fix #5250
	// where a yum install of the `setup` package can fail
	//
	// When /etc/hosts on the host system is unmodified from the distro 'setup' package, yum
	// will try to rename & replace it if the 'setup' package is reinstalled / upgraded. This will
	// fail as it is bind mounted, and cannot be renamed.
	//
	// Adding a newline means the staged file is now different than the one in the 'setup' package
	// and yum will leave the file alone, as it considers it modified.
	content = append(content, []byte("\n")...)

	sessionFile := filepath.Join(b.TmpDir, filepath.Base(destSubpath))
	err = createFileWithContent(sessionFile, content, os.O_CREATE|os.O_WRONLY, 0o666, "staging file")
	if err != nil {
		return "", err
	}

	return sessionFile, nil
}

// Create a file with the specified content
// nameForMsg is used to refer to the file in the error message.
func createFileWithContent(path string, content []byte, flag int, perm fs.FileMode, nameForMsg string) error {
	f, err := os.OpenFile(path, flag, perm)

	if err != nil {
		return fmt.Errorf("failed to create %s: %s", nameForMsg, err)
	}

	if _, err := f.Write(content); err != nil {
		f.Close()
		return fmt.Errorf("failed to write %s: %s", nameForMsg, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %s", nameForMsg, err)
	}

	return nil
}

func createScript(path string, content []byte) error {
	return createFileWithContent(path, content, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755, "script")
}

func getSectionScriptArgs(name string, script string, s types.Script) ([]string, error) {
	args := []string{"/bin/sh", "-ex"}
	// trim potential trailing comment from args and append to args list
	sectionParams := strings.Fields(strings.Split(s.Args, "#")[0])

	commandOption := false

	// look for -c option, we assume that everything after is part of -c
	// arguments and we just inject script path as the last arguments of -c
	for i, param := range sectionParams {
		if param == "-c" {
			if len(sectionParams)-1 < i+1 {
				return nil, fmt.Errorf("bad %s section '-c' parameter: missing arguments", name)
			}
			// replace shell "[args...]" arguments list by single
			// argument "shell [args...] script"
			shellArgs := strings.Join(sectionParams[i+1:], " ")
			sectionParams = append(sectionParams[0:i+1], shellArgs+" "+script)
			commandOption = true
			break
		}
	}

	args = append(args, sectionParams...)
	if !commandOption {
		args = append(args, script)
	}

	return args, nil
}

// currentEnvNoApptainer returns the current environment, minus any APPTAINER_ vars,
// but allowing those specified in the permitted slice. E.g. 'NV' in the permitted slice
// will pass through `APPTAINER_NV`, but strip out `APPTAINER_OTHERVAR`.
func currentEnvNoApptainer(permitted []string) []string {
	envs := make([]string, 0)

	for _, e := range os.Environ() {
		for _, prefix := range env.ApptainerPrefixes {
			if !strings.HasPrefix(e, prefix) {
				envs = append(envs, e)
				break
			}
			envKey := strings.SplitN(e, "=", 2)
			if slice.ContainsString(permitted, strings.TrimPrefix(envKey[0], prefix)) {
				sylog.Debugf("Passing through env var %s to apptainer", e)
				envs = append(envs, e)
				break
			}
		}
	}

	return envs
}
