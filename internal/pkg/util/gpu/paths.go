// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/paths"
)

// gpuliblist returns libraries/binaries listed in a gpu lib list config file, typically
// located in buildcfg.APPTAINER_CONFDIR
func gpuliblist(configFilePath string) ([]string, error) {
	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %v", configFilePath, err)
	}
	defer file.Close()

	var libs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && line[0] != '#' {
			libs = append(libs, line)
		}
	}
	return libs, nil
}

// compat32Paths returns the 32-bit variants of the libraries listed in a gpu
// lib list config file, to be mounted into the container's 32-bit
// compatibility library directory.
func compat32Paths(configFilePath string) ([]string, error) {
	gpuFiles, err := gpuliblist(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %v", filepath.Base(configFilePath), err)
	}
	if len(gpuFiles) == 0 {
		return nil, nil
	}

	return paths.ResolveCompat32(gpuFiles)
}
