// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package args

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
)

func ReadBuildArgs(args []string, argFile string) (map[string]string, error) {
	buildVarsMap := make(map[string]string)
	if argFile != "" {
		file, err := os.Open(argFile)
		if err != nil {
			return buildVarsMap, fmt.Errorf("error while opening file %q: %s", argFile, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			text := scanner.Text()
			k, v, err := getKeyVal(text)
			if err != nil {
				sylog.Warningf("Skipping %q in build arg file: %s", text, err)
				continue
			}

			buildVarsMap[k] = v
		}

		if err := scanner.Err(); err != nil {
			return buildVarsMap, fmt.Errorf("error reading build arg file %q: %s", argFile, err)
		}
	}

	for _, arg := range args {
		k, v, err := getKeyVal(arg)
		if err != nil {
			return nil, err
		}

		buildVarsMap[k] = v
	}

	return buildVarsMap, nil
}

// ReadDefaults reads in the '%arguments' section of (one build stage of) a
// definition file, and returns the default argument values specified in that
// section as a map. If file contained no '%arguments' section, an empty map is
// returned.
func ReadDefaults(def types.Definition) map[string]string {
	defaultArgsMap := make(map[string]string)
	if def.BuildData.Arguments.Script != "" {
		scanner := bufio.NewScanner(strings.NewReader(def.BuildData.Arguments.Script))
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text != "" && !strings.HasPrefix(text, "#") {
				k, v, err := getKeyVal(text)
				if err != nil {
					sylog.Warningf("Skipping %q in 'arguments' section: %s", text, err)
					continue
				}
				defaultArgsMap[k] = v
			}
		}
	}

	return defaultArgsMap
}

func getKeyVal(text string) (string, string, error) {
	if !strings.Contains(text, "=") {
		return "", "", fmt.Errorf("%q is not a key=value pair", text)
	}

	matches := strings.SplitN(text, "=", 2)
	if len(matches) != 2 {
		return "", "", fmt.Errorf("%q is not a key=value pair", text)
	}

	key := strings.TrimSpace(matches[0])
	if key == "" {
		return "", "", fmt.Errorf("missing key portion in %q", text)
	}
	val := strings.TrimSpace(matches[1])
	if val == "" {
		return "", "", fmt.Errorf("missing value portion in %q", text)
	}
	return key, val, nil
}
