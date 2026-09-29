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

// Arch converts a platform architecture into an RPM architecture
func Arch(platformArch, platformVariant string) string {
	switch platformArch {
	case "amd64":
		// RHEL 8: x86_64 with v1 (microarchitecture level 1)
		// RHEL 9: x86_64 with v2 (microarchitecture level 2)
		// RHEL 10: x86_64 with v3 (microarchitecture level 3)
		// - RHEL 10 dropped v2 support (users affected, e.g. at CERN)
		// - AlmaLinux 10: supports both v2 (x86_64_v2) and v3 (x86_64)
		if platformVariant == "v2" {
			return "x86_64_v2"
		}
		return "x86_64"
	case "arm":
		switch platformVariant {
		case "v5":
			return "armv5hl"
		case "v6":
			return "armv6hl"
		case "v7":
			return "armv7hl"
		default:
			return "armv7hl"
		}
	case "arm64":
		return "aarch64"
	default:
		return platformArch
	}
}
