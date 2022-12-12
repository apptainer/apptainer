// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2022 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/sylog"
)

const mainGo = `package main

import (
	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
)

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "%s",
		Author:      "Put your name or mail here",
		Version:     "0.1.0",
		Description: "Put a nice description",
	},
	Callbacks: []pluginapi.Callback{},
	Install:   installCallback,
}

func installCallback(path string) error {
	// Create required stuff during "plugin install"
	// (eg: configuration file, setup ...). Be careful
	// during setup as this callback is executed with
	// root privileges.
	return nil
}

// Write plugin callbacks here and register them in Callbacks
`

const gitIgnore = `apptainer_source
*.sif
*.o
*.a
`

// Create creates a skeleton plugin directory structure
// to start development of a new plugin.
func Create(path, name string) error {
	if buildcfg.IsReproducibleBuild() {
		return fmt.Errorf("plugin functionality is not available in --reproducible builds of apptainer")
	}

	dir, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("could not determine absolute path for %s: %s", path, err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("while creating plugin directory %s: %s", dir, err)
	}

	// create main.go skeleton
	filename := filepath.Join(dir, "main.go")
	content := fmt.Sprintf(mainGo, name)
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return fmt.Errorf("while creating plugin %s: %s", filename, err)
	}

	// create .gitignore skeleton
	filename = filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(filename, []byte(gitIgnore), 0o644); err != nil {
		return fmt.Errorf("while creating plugin %s: %s", filename, err)
	}

	// create symlink to apptainer source directory
	sourceLink := filepath.Join(dir, ApptainerSource)

	if _, err := os.Stat(buildcfg.SOURCEDIR); os.IsNotExist(err) {
		ls := fmt.Sprintf("ln -s /path/to/apptainer/source %s", sourceLink)
		sylog.Warningf("Apptainer source %s doesn't exist, you would have to execute manually %q", buildcfg.SOURCEDIR, ls)
		return nil
	} else if err != nil {
		return fmt.Errorf("while getting %s information: %s", sourceLink, err)
	}

	// delete it first if already present
	if err := os.Remove(sourceLink); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("while removing %s symlink: %s", sourceLink, err)
		}
	}

	if err := os.Symlink(buildcfg.SOURCEDIR, sourceLink); err != nil {
		return fmt.Errorf("while creating symlink %s: %s", sourceLink, err)
	}

	return nil
}
