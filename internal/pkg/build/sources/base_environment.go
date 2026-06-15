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

	"github.com/apptainer/apptainer/pkg/build/types"
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

// mkdirAllInRootfs creates dir, and any missing parents, within the bundle
// rootfs. If the final path already exists it is left untouched. This tolerates
// images that provide one of these paths as a symlink (e.g. var/tmp -> /tmp);
// os.Root.MkdirAll returns an error for an existing symlink leaf, whereas the
// plain os.MkdirAll this replaced resolved and accepted it.
func mkdirAllInRootfs(b *types.Bundle, dir string) error {
	if _, err := b.Rootfs.Lstat(dir); err == nil {
		return nil
	}
	return b.Rootfs.MkdirAll(dir, 0o755)
}

func makeDirs(b *types.Bundle) error {
	dirs := []string{
		filepath.Join(".singularity.d", "libs"),
		filepath.Join(".singularity.d", "actions"),
		filepath.Join(".singularity.d", "env"),
		"dev",
		"proc",
		"root",
		filepath.Join("var", "tmp"),
		"tmp",
		"etc",
		"sys",
		"home",
	}
	for _, dir := range dirs {
		if err := mkdirAllInRootfs(b, dir); err != nil {
			return err
		}
	}
	return nil
}

func makeSymlinks(b *types.Bundle) error {
	if _, err := b.Rootfs.Stat("singularity"); err != nil {
		if err = b.Rootfs.Symlink("/.singularity.d/runscript", "singularity"); err != nil {
			return err
		}
	}
	if _, err := b.Rootfs.Stat(".run"); err != nil {
		if err = b.Rootfs.Symlink("/.singularity.d/actions/run", ".run"); err != nil {
			return err
		}
	}
	if _, err := b.Rootfs.Stat(".exec"); err != nil {
		if err = b.Rootfs.Symlink("/.singularity.d/actions/exec", ".exec"); err != nil {
			return err
		}
	}
	if _, err := b.Rootfs.Stat(".test"); err != nil {
		if err = b.Rootfs.Symlink("/.singularity.d/actions/test", ".test"); err != nil {
			return err
		}
	}
	if _, err := b.Rootfs.Stat(".shell"); err != nil {
		if err = b.Rootfs.Symlink("/.singularity.d/actions/shell", ".shell"); err != nil {
			return err
		}
	}
	if _, err := b.Rootfs.Stat("environment"); err != nil {
		if err = b.Rootfs.Symlink("/.singularity.d/env/90-environment.sh", "environment"); err != nil {
			return err
		}
	}
	return nil
}

func makeFile(b *types.Bundle, name string, perm os.FileMode, s string, overwrite bool) (err error) {
	// #4532 - If the file already exists ensure it has requested permissions
	// as OpenFile won't set on an existing file and some docker
	// containers have hosts or resolv.conf without write perm.
	if info, statErr := b.Rootfs.Stat(name); statErr == nil && info.Mode().IsRegular() {
		if err = b.Rootfs.Chmod(name, perm); err != nil {
			return
		}
		if !overwrite {
			sylog.Debugf("Will not write to %s file due to existence of file and overwrite flag is set to false", name)
			return
		}
	}
	// Create the file if it's not in the container, or truncate and write s
	// into it otherwise.
	f, err := b.Rootfs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.WriteString(s)
	return
}

func makeFiles(b *types.Bundle, overwrite bool) error {
	if err := makeFile(b, filepath.Join("etc", "hosts"), 0o644, "", overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join("etc", "resolv.conf"), 0o644, "", overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "actions", "exec"), 0o755, execFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "actions", "run"), 0o755, runFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "actions", "shell"), 0o755, shellFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "actions", "start"), 0o755, startFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "actions", "test"), 0o755, testFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "env", "01-base.sh"), 0o755, baseShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "env", "90-environment.sh"), 0o755, environmentShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "env", "95-apps.sh"), 0o755, appsShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "env", "99-base.sh"), 0o755, base99ShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "env", "99-runtimevars.sh"), 0o755, base99runtimevarsShFileContent, overwrite); err != nil {
		return err
	}
	if err := makeFile(b, filepath.Join(".singularity.d", "runscript"), 0o755, runscriptFileContent, overwrite); err != nil {
		return err
	}
	return makeFile(b, filepath.Join(".singularity.d", "startscript"), 0o755, startscriptFileContent, overwrite)
}

// makeBaseEnv inserts Apptainer specific directories, symlinks, and files
// into the container rootfs. If overwrite is true, then any existing files
// will be overwritten with new content. If overwrite is false, existing files
// (e.g. where the rootfs has been extracted from an existing image) will not be
// modified.
func makeBaseEnv(b *types.Bundle, overwrite bool) (err error) {
	var info os.FileInfo

	// Ensure we can write into the root of rootPath
	if info, err = b.Rootfs.Stat("."); err != nil {
		err = fmt.Errorf("build: failed to stat rootPath: %v", err)
		return err
	}
	if info.Mode()&0o200 == 0 {
		sylog.Infof("Adding owner write permission to build path: %s\n", b.RootfsPath)
		if err = b.Rootfs.Chmod(".", info.Mode()|0o200); err != nil {
			err = fmt.Errorf("build: failed to make rootPath writable: %v", err)
			return err
		}
	}

	if err = makeDirs(b); err != nil {
		err = fmt.Errorf("build: failed to make environment dirs: %v", err)
		return err
	}
	if err = makeSymlinks(b); err != nil {
		err = fmt.Errorf("build: failed to make environment symlinks: %v", err)
		return err
	}
	if err = makeFiles(b, overwrite); err != nil {
		err = fmt.Errorf("build: failed to make environment files: %v", err)
		return err
	}

	return err
}
