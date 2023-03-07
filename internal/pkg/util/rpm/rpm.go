// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package rpm

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrMacroUndefined = errors.New("macro is not defined")

// GetMacro returns the value of the provided macro name
func GetMacro(name string) (value string, err error) {
	rpm, err := exec.LookPath("rpm")
	if err != nil {
		return "", fmt.Errorf("rpm command not found: %w", err)
	}

	args := []string{"--eval", "%{" + name + "}"}
	cmd := exec.Command(rpm, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("while looking up value of rpm macro %s: %s", name, err)
	}

	eval := strings.TrimSuffix(string(out), "\n")
	if eval == "%{"+name+"}" {
		return "", ErrMacroUndefined
	}
	return eval, nil
}
