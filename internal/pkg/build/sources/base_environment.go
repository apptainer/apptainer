// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// Contents of /.singularity.d/actions/exec
//
//go:embed exec.sh
var execFileContent string

// Contents of /.singularity.d/actions/run
//
//go:embed run.sh
var runFileContent string

// Contents of /.singularity.d/actions/shell
//
//go:embed shell.sh
var shellFileContent string

// Contents of /.singularity.d/actions/start
//
//go:embed start.sh
var startFileContent string

// Contents of /.singularity.d/actions/test
//
//go:embed test.sh
var testFileContent string

// Contents of /.singularity.d/env/01-base.sh
//
//go:embed 01-base.sh
var baseShFileContent string

// Contents of /.singularity.d/env/90-environment.sh and /.singularity.d/env/91-environment.sh
//
//go:embed 90-environment.sh
var environmentShFileContent string

// Contents of /.singularity.d/env/95-apps.sh
//
//go:embed 95-apps.sh
var appsShFileContent string

// Contents of /.singularity.d/env/99-base.sh
//
//go:embed 99-base.sh
var base99ShFileContent string

// Contents of /.singularity.d/env/99-runtimevars.sh
//
//go:embed 99-runtimevars.sh
var base99runtimevarsShFileContent string

// Contents of /.singularity.d/runscript
//
//go:embed runscript.sh
var runscriptFileContent string

// Contents of /.singularity.d/startscript
//
//go:embed startscript.sh
var startscriptFileContent string

func makeDirs(rootPath string) error {
	if err := os.MkdirAll(filepath.Join(rootPath, ".singularity.d", "libs"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, ".singularity.d", "actions"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, ".singularity.d", "env"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "dev"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "proc"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "root"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "var", "tmp"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "tmp"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "etc"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "sys"), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(rootPath, "home"), 0o755)
}

func makeSymlinks(rootPath string) error {
	if _, err := os.Stat(filepath.Join(rootPath, "singularity")); err != nil {
		if err = os.Symlink(".singularity.d/runscript", filepath.Join(rootPath, "singularity")); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(rootPath, ".run")); err != nil {
		if err = os.Symlink(".singularity.d/actions/run", filepath.Join(rootPath, ".run")); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(rootPath, ".exec")); err != nil {
		if err = os.Symlink(".singularity.d/actions/exec", filepath.Join(rootPath, ".exec")); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(rootPath, ".test")); err != nil {
		if err = os.Symlink(".singularity.d/actions/test", filepath.Join(rootPath, ".test")); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(rootPath, ".shell")); err != nil {
		if err = os.Symlink(".singularity.d/actions/shell", filepath.Join(rootPath, ".shell")); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(rootPath, "environment")); err != nil {
		if err = os.Symlink(".singularity.d/env/90-environment.sh", filepath.Join(rootPath, "environment")); err != nil {
			return err
		}
	}
	return nil
}

func makeFile(name string, perm os.FileMode, s string, overwrite bool) (err error) {
	// #4532 - If the file already exists ensure it has requested permissions
	// as OpenFile won't set on an existing file and some docker
	// containers have hosts or resolv.conf without write perm.
	if fs.IsFile(name) {
		if err = os.Chmod(name, perm); err != nil {
			return
		}
		if !overwrite {
			sylog.Debugf("Will not write to %s file due to existence of file and overwrite flag is set to false", name)
			return
		}
	}
	// Create the file if it's not in the container, or truncate and write s
	// into it otherwise.
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.WriteString(s)
	return
}

func makeFiles(rootPath string, overwrite bool) error {
	if err := makeFile(filepath.Join(rootPath, "etc", "hosts"), 0o644, "", overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, "etc", "resolv.conf"), 0o644, "", overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "actions", "exec"), 0o755, execFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "actions", "run"), 0o755, runFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "actions", "shell"), 0o755, shellFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "actions", "start"), 0o755, startFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "actions", "test"), 0o755, testFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "env", "01-base.sh"), 0o755, baseShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "env", "90-environment.sh"), 0o755, environmentShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "env", "95-apps.sh"), 0o755, appsShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "env", "99-base.sh"), 0o755, base99ShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "env", "99-runtimevars.sh"), 0o755, base99runtimevarsShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(filepath.Join(rootPath, ".singularity.d", "runscript"), 0o755, runscriptFileContent, overwrite); err != nil {
		return err
	}
	return makeFile(filepath.Join(rootPath, ".singularity.d", "startscript"), 0o755, startscriptFileContent, overwrite)
}

// makeBaseEnv inserts Apptainer specific directories, symlinks, and files
// into the container rootfs. If overwrite is true, then any existing files
// will be overwritten with new content. If overwrite is false, existing files
// (e.g. where the rootfs has been extracted from an existing image) will not be
// modified.
func makeBaseEnv(rootPath string, overwrite bool) (err error) {
	var info os.FileInfo

	// Ensure we can write into the root of rootPath
	if info, err = os.Stat(rootPath); err != nil {
		err = fmt.Errorf("build: failed to stat rootPath: %v", err)
		return err
	}
	if info.Mode()&0o200 == 0 {
		sylog.Infof("Adding owner write permission to build path: %s\n", rootPath)
		if err = os.Chmod(rootPath, info.Mode()|0o200); err != nil {
			err = fmt.Errorf("build: failed to make rootPath writable: %v", err)
			return err
		}
	}

	if err = makeDirs(rootPath); err != nil {
		err = fmt.Errorf("build: failed to make environment dirs: %v", err)
		return err
	}
	if err = makeSymlinks(rootPath); err != nil {
		err = fmt.Errorf("build: failed to make environment symlinks: %v", err)
		return err
	}
	if err = makeFiles(rootPath, overwrite); err != nil {
		err = fmt.Errorf("build: failed to make environment files: %v", err)
		return err
	}

	return err
}
