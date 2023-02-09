// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/blang/semver/v4"
)

// ApptainerSource represents the symlink name which will
// point to the Apptainer source directory.
const ApptainerSource = "apptainer_source"

// Module describes a Go module with its corresponding path and version.
type Module struct {
	Path    string
	Version string
}

// String returns the string representation of a module.
func (m Module) String() string {
	if m.Version != "" {
		return m.Path + " " + m.Version
	}
	return m.Path
}

// GoMod describes a parsed go.mod file.
type GoMod struct {
	Module  Module
	Go      string
	Require []Require
	Exclude []Module
	Replace []Replace
}

// GetReplace returns the replace record for the
// provided module path.
func (g GoMod) GetReplace(path string) *Replace {
	for _, r := range g.Replace {
		if r.Old.Path == path {
			return &r
		}
	}
	return nil
}

// GetRequire returns the require record for the
// provided module path.
func (g GoMod) GetRequire(path string) *Require {
	for _, r := range g.Require {
		if r.Path == path {
			return &r
		}
	}
	return nil
}

// GetExclude returns the exclude record for the
// provided module path.
func (g GoMod) GetExclude(path string) *Module {
	for _, e := range g.Exclude {
		if e.Path == path {
			return &e
		}
	}
	return nil
}

// Require describes a require directive in go.mod files.
type Require struct {
	Path     string
	Version  string
	Indirect bool
}

// String returns the string representation of a require line.
func (r Require) String() string {
	indirect := ""
	if r.Indirect {
		indirect = " // indirect"
	}
	if r.Version != "" {
		return r.Path + " " + r.Version + indirect
	}
	return r.Path + indirect
}

// Replace describes a replace directive in go.mod files.
type Replace struct {
	Old Module
	New Module
}

// String returns the string representation of a replace line.
func (r Replace) String() string {
	return r.Old.String() + " => " + r.New.String()
}

// GetModules parses the go.mod file found in directory and returns
// a GoMod instance.
func GetModules(dir string) (*GoMod, error) {
	var b bytes.Buffer
	var e bytes.Buffer

	goMod := filepath.Join(dir, "go.mod")

	if _, err := os.Stat(goMod); err != nil {
		return nil, fmt.Errorf("while getting information for %s: %s", goMod, err)
	}

	goPath, err := bin.FindBin("go")
	if err != nil {
		return nil, fmt.Errorf("while retrieving go command path: %s", err)
	}

	cmd := exec.Command(goPath, "mod", "edit", "-json", goMod)
	cmd.Stdout = &b
	cmd.Stderr = &e

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("while reading %s: %s\nCommand error:\n%s", goMod, err, e.String())
	}

	modules := new(GoMod)

	if err := json.NewDecoder(&b).Decode(modules); err != nil {
		return nil, fmt.Errorf("while decoding json data: %s", err)
	}

	return modules, nil
}

// PrepareGoModules returns a byte array containing a generated go.mod matching
// Apptainer modules in use in order to compile/load the plugin with same version
// of dependencies.
func PrepareGoModules(pluginDir string, disableMinorCheck bool) ([]byte, error) {
	var goMod bytes.Buffer

	if buildcfg.IsReproducibleBuild() {
		return nil, fmt.Errorf("plugin functionality is not available in --reproducible builds of apptainer")
	}

	singModules, err := GetModules(buildcfg.SOURCEDIR)
	if err != nil {
		return nil, fmt.Errorf("while getting Apptainer Go modules: %s", err)
	}
	apptainerPackage := singModules.Module.Path

	pluginModules, err := GetModules(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("while getting plugin Go modules: %s", err)
	}

	fmt.Fprintf(&goMod, "module %s\n\n", pluginModules.Module.Path)
	fmt.Fprintf(&goMod, "go %s\n\n", singModules.Go)

	for i, r := range pluginModules.Require {
		if i == 0 {
			fmt.Fprintf(&goMod, "require (\n")
		}

		if sr := singModules.GetRequire(r.Path); sr != nil && r.Version != sr.Version {
			sylog.Infof("Replacing %q by %q", r, sr)
			if err := checkCompatibility(r.Version, sr.Version, disableMinorCheck); err != nil {
				return nil, fmt.Errorf("package %q error: %s", r.Path, err)
			}
			r.Version = sr.Version
		} else if r.Path == apptainerPackage {
			// force apptainer version to v0.0.0
			r.Version = "v0.0.0"
		}

		if sr := singModules.GetExclude(r.Path); sr != nil && sr.Version == r.Version {
			return nil, fmt.Errorf("plugin requires %q but it's excluded by apptainer go.mod %q", r, sr)
		}
		if sr := singModules.GetReplace(r.Path); sr != nil && sr.New.Version != r.Version {
			return nil, fmt.Errorf("plugin requires %q but it's replaced by apptainer go.mod %q", r, sr)
		}

		fmt.Fprintf(&goMod, "\t%s\n", r)

		if i == len(pluginModules.Require)-1 {
			fmt.Fprintf(&goMod, ")\n\n")
		}
	}

	fmt.Fprintf(&goMod, "replace (\n")
	fmt.Fprintf(&goMod, "\t%s => ./%s\n", apptainerPackage, ApptainerSource)

	// inject apptainer replace first
	for _, r := range singModules.Replace {
		fmt.Fprintf(&goMod, "\t%s\n", r)
	}

	for _, r := range pluginModules.Replace {
		if sr := singModules.GetReplace(r.Old.Path); sr != nil {
			if sr.New.Version == r.New.Version && sr.New.Path == r.New.Path {
				continue
			}
			return nil, fmt.Errorf("plugin go.mod contains replace %q while apptainer replaced it with %q", r, sr)
		} else if r.Old.Path == apptainerPackage {
			// previously added above as first replace
			continue
		}

		if sr := singModules.GetRequire(r.Old.Path); sr != nil {
			if r.New.Path != sr.Path {
				return nil, fmt.Errorf("plugin go.mod contains replace %q while apptainer requires it with %q", r, sr)
			}
		}

		fmt.Fprintf(&goMod, "\t%s\n", r)
	}

	fmt.Fprintf(&goMod, ")\n\n")

	for i, r := range pluginModules.Exclude {
		if i == 0 {
			fmt.Fprintf(&goMod, "exclude (\n")
		}

		// check for version incompatibilities in
		// apptainer required and replaced packages
		if sr := singModules.GetRequire(r.Path); sr != nil {
			if sr.Version != r.Version {
				return nil, fmt.Errorf("apptainer go.mod contains require %q incompatible with plugin exclude %q", sr, r)
			}
		}
		if sr := singModules.GetReplace(r.Path); sr != nil {
			if sr.New.Version != r.Version {
				return nil, fmt.Errorf("apptainer go.mod contains replace %q incompatible with plugin exclude %q", sr, r)
			}
		}

		fmt.Fprintf(&goMod, "\t%s\n", r)

		if i == len(pluginModules.Exclude)-1 {
			fmt.Fprintf(&goMod, ")\n\n")
		}
	}

	return goMod.Bytes(), nil
}

func checkCompatibility(pv string, sv string, disableMinorCheck bool) error {
	pluginVer, err := semver.Make(pv[1:])
	if err != nil {
		return fmt.Errorf("plugin version %s is not a semantic version: %s", pv, err)
	}
	apptainerVer, err := semver.Make(sv[1:])
	if err != nil {
		return fmt.Errorf("apptainer version %s is not a semantic version: %s", sv, err)
	}

	// if major version doesn't match we abort
	if pluginVer.Major != apptainerVer.Major {
		return fmt.Errorf("incompatible major version, plugin %s / apptainer %s", pv, sv)
	}

	// if the plugin package version is > to Apptainer package
	// version the backward compatibility is not valid and possible
	// failures may occur at compilation, we abort in this case
	if !disableMinorCheck && pluginVer.GT(apptainerVer) {
		return fmt.Errorf("plugin expect a more recent minor version %s while apptainer uses %s", pv, sv)
	}

	// at this point we assume that Apptainer
	// package version is backward compatible
	// with the one used by the plugin
	return nil
}
