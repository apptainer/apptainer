// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package syfs provides functions to access apptainer's file system
// layout.
package syfs

import (
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// Configuration files/directories.
const (
	RemoteConfFile         = "remote.yaml"
	RemoteCache            = "remote-cache"
	DockerConfFile         = "docker-config.json"
	apptainerDir           = ".apptainer"
	legacyDir              = ".singularity"
	defaultLocalKeyDirName = "keys" // defaultLocalKeyDirName represents the default local key storage folder name
)

// cache contains the information for the current user
var cache struct {
	sync.Once
	configDir string // apptainer user configuration directory
}

// ConfigDir returns the directory where the apptainer user
// configuration and data is located.
func ConfigDir() string {
	cache.Do(func() {
		cache.configDir = configDir(apptainerDir)
		sylog.Debugf("Using apptainer directory %q", cache.configDir)
	})

	return cache.configDir
}

func configDir(dir string) string {
	envKey := "CONFIGDIR"
	configDir := env.GetenvLegacy(envKey, envKey)
	if configDir != "" {
		return configDir
	}

	homedir := os.Getenv("HOME")
	if homedir == "" {
		user, err := user.Current()
		if err != nil {
			sylog.Warningf("Could not lookup the current user's information: %s", err)

			cwd, err := os.Getwd()
			if err != nil {
				sylog.Warningf("Could not get current working directory: %s", err)
				return dir
			}
			homedir = cwd
		} else {
			homedir = user.HomeDir
		}
	}

	return filepath.Join(homedir, dir)
}

func RemoteConf() string {
	return filepath.Join(ConfigDir(), RemoteConfFile)
}

func RemoteCacheDir() string {
	return filepath.Join(ConfigDir(), RemoteCache)
}

func DockerConf() string {
	return filepath.Join(ConfigDir(), DockerConfFile)
}

func FallbackDockerConf() string {
	return filepath.Join(configDir(".docker"), "config.json")
}

// ConfigDirForUsername returns the directory where the apptainer
// configuration and data for the specified username is located.
func ConfigDirForUsername(username string) (string, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return "", err
	}

	if cu, err := user.Current(); err == nil && u.Username == cu.Username {
		return ConfigDir(), nil
	}

	return filepath.Join(u.HomeDir, apptainerDir), nil
}

// LegacyConfigDir returns where singularity stores user configuration.
// NOTE: this location should only be used for migration/compatibility and
// never written to by apptainer.
func LegacyConfigDir() string {
	return configDir(legacyDir)
}

// LegacyRemoteConf returns where singularity stores user remote configuration.
// NOTE: this location should only be used for migration/compatibility and
// never written to by apptainer.
func LegacyRemoteConf() string {
	return filepath.Join(LegacyConfigDir(), RemoteConfFile)
}

// LegacyDockerConf returns where singularity stores user oci registry configuration.
// NOTE: this location should only be used for migration/compatibility and
// never written to by apptainer.
func LegacyDockerConf() string {
	return filepath.Join(LegacyConfigDir(), DockerConfFile)
}

func DefaultLocalKeyDirPath() string {
	// read this as: look for APPTAINER_KEYSDIR and/or SINGULARITY_SYPGPDIR
	if dir := env.GetenvLegacy("KEYSDIR", "SYPGPDIR"); dir != "" {
		return dir
	}
	return filepath.Join(ConfigDir(), defaultLocalKeyDirName)
}
