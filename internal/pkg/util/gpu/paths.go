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
	"strings"
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
