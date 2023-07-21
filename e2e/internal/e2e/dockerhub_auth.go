// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
)

const dockerHub = "docker.io"

func SetupDockerHubCredentials(t *testing.T) {
	var unprivUser, privUser *user.User

	username := os.Getenv("E2E_DOCKER_USERNAME")
	pass := os.Getenv("E2E_DOCKER_PASSWORD")

	if username == "" && pass == "" {
		t.Log("No DockerHub credentials supplied, DockerHub rate limits could be hit")
		return
	}

	unprivUser = CurrentUser(t)
	writeDockerHubCredentials(t, unprivUser.Dir, username, pass)
	Privileged(func(t *testing.T) {
		privUser = CurrentUser(t)
		writeDockerHubCredentials(t, privUser.Dir, username, pass)
	})(t)
}

func writeDockerHubCredentials(t *testing.T, dir, username, pass string) {
	configPath := filepath.Join(dir, ".apptainer", syfs.DockerConfFile)

	cf := configfile.ConfigFile{
		AuthConfigs: map[string]types.AuthConfig{
			dockerHub: {
				Username: username,
				Password: pass,
			},
		},
	}

	configData, err := json.Marshal(cf)
	if err != nil {
		t.Error(err)
	}

	os.WriteFile(configPath, configData, 0o600)
}
