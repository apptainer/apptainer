// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package env

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/shell/interpreter"
	"github.com/apptainer/apptainer/pkg/sylog"
)

const (
	// DefaultPath defines default value for PATH environment variable.
	DefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	// ApptainerPrefix Apptainer environment variable recognized prefixes for Apptainer CLI
	ApptainerPrefix = "APPTAINER_"

	// ApptainerEnvPrefix Apptainer environment variables recognized prefixes for passthru to container
	ApptainerEnvPrefix = "APPTAINERENV_"

	// Legacy singularity prefix
	LegacySingularityPrefix = "SINGULARITY_"

	// Legacy singularity env prefix
	LegacySingularityEnvPrefix = "SINGULARITYENV_"
)

// ApptainerPrefixes the following prefixes are for settings looked at by Apptainer command
var ApptainerPrefixes = []string{ApptainerPrefix, LegacySingularityPrefix}

// ApptainerEnvPrefixes defines the environment variable prefixes for passthru
// to container
var ApptainerEnvPrefixes = []string{ApptainerEnvPrefix, LegacySingularityEnvPrefix}

var ReadOnlyVars = map[string]bool{
	"EUID":   true,
	"GID":    true,
	"HOME":   true,
	"IFS":    true,
	"OPTIND": true,
	"PWD":    true,
	"UID":    true,
}

// SetFromList sets environment variables from environ argument list.
func SetFromList(environ []string) error {
	for _, env := range environ {
		splitted := strings.SplitN(env, "=", 2)
		if len(splitted) != 2 {
			return fmt.Errorf("can't process environment variable %s", env)
		}
		if err := os.Setenv(splitted[0], splitted[1]); err != nil {
			return err
		}
	}
	return nil
}

// GetenvLegacy retrieves environment variables value from both
// APPTAINER_ and SINGULARITY_ and display warning accordingly
// if the old SINGULARITY_ prefix is used. APPTAINER_ prefixed
// variable always take precedence if not empty.
func GetenvLegacy(key, legacyKey string) string {
	keyEnv := ApptainerPrefixes[0] + key
	legacyKeyEnv := ApptainerPrefixes[1] + legacyKey

	val := os.Getenv(keyEnv)
	if val == "" {
		val = os.Getenv(legacyKeyEnv)
		if val != "" {
			sylog.Infof("Environment variable %v is set, but %v is preferred", legacyKeyEnv, keyEnv)
		}
	} else if os.Getenv(legacyKeyEnv) != "" && os.Getenv(legacyKeyEnv) != val {
		sylog.Warningf("%s and %s have different values, using the latter", legacyKeyEnv, keyEnv)
	}

	return val
}

// TrimApptainerKey returns the key without APPTAINER_ prefix.
func TrimApptainerKey(key string) string {
	return strings.TrimPrefix(key, ApptainerPrefixes[0])
}

// FileMap returns a map of KEY=VAL env vars from an environment file f. The env
// file is shell evaluated using mvdan/sh with arguments and environment set
// from args and hostEnv.
func FileMap(ctx context.Context, f string, args []string, hostEnv []string) (map[string]string, error) {
	envMap := map[string]string{}

	content, err := os.ReadFile(f)
	if err != nil {
		return envMap, fmt.Errorf("could not read environment file %q: %w", f, err)
	}

	// Use the embedded shell interpreter to evaluate the env file, with an empty starting environment.
	// Shell takes care of comments, quoting etc. for us and keeps compatibility with native runtime.
	env, err := interpreter.EvaluateEnv(ctx, content, args, hostEnv)
	if err != nil {
		return envMap, fmt.Errorf("while processing %s: %w", f, err)
	}

	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) < 2 {
			continue
		}
		// Strip out the runtime env vars set by the shell interpreter so we
		// don't attempt to overwrite bash builtin readonly vars.
		// https://github.com/sylabs/singularity/issues/1263
		if _, ok := ReadOnlyVars[parts[0]]; ok {
			continue
		}

		envMap[parts[0]] = parts[1]
	}

	return envMap, nil
}

// MergeMap merges two maps of environment variables, with values in b replacing
// values also set in a.
func MergeMap(a map[string]string, b map[string]string) map[string]string {
	for k, v := range b {
		a[k] = v
	}
	return a
}
