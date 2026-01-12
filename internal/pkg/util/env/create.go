// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package env

import (
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/pkg/sylog"
)

var alwaysPassKeys = map[string]struct{}{
	"TERM":        {},
	"http_proxy":  {},
	"HTTP_PROXY":  {},
	"https_proxy": {},
	"HTTPS_PROXY": {},
	"no_proxy":    {},
	"NO_PROXY":    {},
	"all_proxy":   {},
	"ALL_PROXY":   {},
	"ftp_proxy":   {},
	"FTP_PROXY":   {},
}

// boolean value defines if the variable could be overridden
// with the APPTAINERENV_ or SINGULARITYENV_ variant.
var alwaysOmitKeys = map[string]bool{
	"HOME":                false,
	"PATH":                false,
	"APPTAINER_SHELL":     false,
	"APPTAINER_APPNAME":   false,
	"SINGULARITY_SHELL":   false,
	"SINGULARITY_APPNAME": false,
	"LD_LIBRARY_PATH":     true,
}

type envKeyMap = map[string]string

// setKeyIfNotAlreadyOverridden sets a value for key if not already overridden
func setKeyIfNotAlreadyOverridden(g *generate.Generator, envKeys envKeyMap, prefixedKey, key, value string) {
	if oldValue, ok := envKeys[key]; ok {
		if oldValue != value {
			sylog.Warningf("Skipping environment variable [%s=%s], %s is already overridden with different value [%s]", prefixedKey, value, key, oldValue)
		} else {
			sylog.Debugf("Skipping environment variable [%s=%s], %s is already overridden with the same value", prefixedKey, value, key)
		}
	} else {
		sylog.Verbosef("Forwarding %s as %s environment variable", prefixedKey, key)
		envKeys[key] = value
		g.RemoveProcessEnv(key)
	}
}

// overridesForContainerEnv sets all environment variables which have overrides.
func overridesForContainerEnv(g *generate.Generator, hostEnvs []string) envKeyMap {
	envKeys := make(envKeyMap)
	for _, prefix := range ApptainerEnvPrefixes {
		for _, env := range hostEnvs {
			if strings.HasPrefix(env, prefix) {
				e := strings.SplitN(env, "=", 2)
				if len(e) != 2 {
					sylog.Verbosef("Can't process override environment variable %s", env)
				} else {
					key := e[0][len(prefix):]
					if key != "" {
						switch key {
						case "PREPEND_PATH":
							setKeyIfNotAlreadyOverridden(g, envKeys, e[0], "SING_USER_DEFINED_PREPEND_PATH", e[1])
						case "APPEND_PATH":
							setKeyIfNotAlreadyOverridden(g, envKeys, e[0], "SING_USER_DEFINED_APPEND_PATH", e[1])
						case "PATH":
							setKeyIfNotAlreadyOverridden(g, envKeys, e[0], "SING_USER_DEFINED_PATH", e[1])
						default:
							if permitted, ok := alwaysOmitKeys[key]; ok && !permitted {
								sylog.Warningf("Overriding %s environment variable with %s is not permitted", key, e[0])
								continue
							}
							setKeyIfNotAlreadyOverridden(g, envKeys, e[0], key, e[1])
						}
					}
				}
			}
		}
	}
	return envKeys
}

// warning if deprecated keys are set
func warnDeprecatedEnvUsage(hostEnvs []string) {
	envMap := make(map[string]string)
	for _, env := range hostEnvs {
		strs := strings.SplitN(env, "=", 2)
		if len(strs) == 2 {
			envMap[strs[0]] = strs[1]
		}
	}
	for _, env := range hostEnvs {
		if strings.HasPrefix(env, LegacySingularityEnvPrefix) {
			strs := strings.SplitN(env, "=", 2)
			if len(strs) == 2 {
				key := strs[0][len(LegacySingularityEnvPrefix):]
				value := strs[1]
				if key != "" {
					legacyEnv := LegacySingularityEnvPrefix + key
					newEnv := ApptainerEnvPrefix + key
					if val, ok := envMap[newEnv]; ok {
						if val != value {
							sylog.Warningf("%s and %s have different values, using the latter", legacyEnv, newEnv)
						}
					} else {
						sylog.Infof("Environment variable %v is set, but %v is preferred", legacyEnv, newEnv)
					}
				}
			}
		}
	}
}

// SetContainerEnv cleans environment variables before running the container.
func SetContainerEnv(g *generate.Generator, hostEnvs []string, noEnv []string,
	cleanEnv bool, homeDest string,
) map[string]string {
	// allow override with APPTAINERENV_LANG
	if cleanEnv {
		g.SetProcessEnv("LANG", "C")
	}

	// process overrides first, order of prefix within the slice of prefixes
	// determines the precedence between various prefixes
	warnDeprecatedEnvUsage(hostEnvs)
	envKeys := overridesForContainerEnv(g, hostEnvs)

	// Add the noEnv keys to the list of variables to omit.
	// Allow them to be overridden if they're newly added.
	for _, noenv := range noEnv {
		if _, ok := alwaysOmitKeys[noenv]; !ok {
			sylog.Debugf("Adding %s to list of variables to skip forwarding", noenv)
			alwaysOmitKeys[noenv] = true
		}
	}

EnvKeys:
	for _, env := range hostEnvs {
		e := strings.SplitN(env, "=", 2)
		if len(e) != 2 {
			sylog.Verbosef("Can't process environment variable %s", env)
			continue EnvKeys
		}

		// APPTAINER_ prefixed environment variables are not forwarded
		for _, prefix := range ApptainerPrefixes {
			if strings.HasPrefix(env, prefix) {
				sylog.Verbosef("Not forwarding %s environment variable", e[0])
				continue EnvKeys
			}
		}

		// APPTAINERENV_ prefixed environment variables will take
		// precedence over the non prefixed variables
		for _, prefix := range ApptainerEnvPrefixes {
			if strings.HasPrefix(env, prefix) {
				// already processed overrides
				continue EnvKeys
			}
		}

		// non prefixed environment variables
		if mustAddToProcessEnv(e[0], cleanEnv) {
			if value, ok := envKeys[e[0]]; ok {
				if value != e[1] {
					sylog.Debugf("Environment variable %s already has value [%s], will not forward new value [%s] from parent process environment", e[0], value, e[1])
				} else {
					sylog.Debugf("Environment variable %s already has duplicate value [%s], will not forward from parent process environment", e[0], value)
				}
			} else {
				// transpose host env variables into config
				sylog.Debugf("Forwarding %s environment variable", e[0])
				g.SetProcessEnv(e[0], e[1])
			}
		}
	}

	sylog.Verbosef("Setting HOME=%s", homeDest)
	sylog.Verbosef("Setting PATH=%s", DefaultPath)
	g.SetProcessEnv("HOME", homeDest)
	g.SetProcessEnv("PATH", DefaultPath)

	return envKeys
}

// mustAddToProcessEnv processes given key and returns if the environment
// variable should be added to the container or not.
func mustAddToProcessEnv(key string, cleanEnv bool) bool {
	if _, ok := alwaysPassKeys[key]; ok {
		return true
	}
	if _, ok := alwaysOmitKeys[key]; ok || cleanEnv {
		return false
	}
	return true
}
