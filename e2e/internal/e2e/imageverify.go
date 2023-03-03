// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test/tool/exec"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/buger/jsonparser"
)

// ImageVerify checks for an image integrity.
func (env TestEnv) ImageVerify(t *testing.T, imagePath string, profile Profile) {
	env.RunApptainer(
		t,
		AsSubtest("BasicIntegrityTests"),
		WithProfile(profile),
		WithCommand("exec"),
		WithArgs(imagePath, "/bin/sh", "-c",
			"test -f /.singularity.d/runscript -a "+
				"-f /.singularity.d/env/01-base.sh -a "+
				"-f /.singularity.d/actions/shell -a "+
				"-f /.singularity.d/actions/exec -a "+
				"-f /.singularity.d/actions/run -a "+
				"-L /environment -a "+
				"-L /singularity"),
		ExpectExit(0),
	)

	tests := []struct {
		name      string
		jsonPath  []string
		expectOut string
	}{
		{
			name:      "LabelCheckType",
			jsonPath:  []string{"type"},
			expectOut: "container",
		},
		{
			// name:      "LabelCheckSchemaVersion",
			jsonPath:  []string{"data", "attributes", "labels", "org.label-schema.schema-version"},
			expectOut: "1.0",
		},
	}

	verifyOutput := func(t *testing.T, r *ApptainerCmdResult) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				jsonOut, err := jsonparser.GetString(r.Stdout, tt.jsonPath...)
				if err != nil {
					t.Fatalf("unable to get expected output from json: %v", err)
				}
				if jsonOut != tt.expectOut {
					t.Fatalf("unexpected failure: got: '%s', expecting: '%s'", jsonOut, tt.expectOut)
				}
			})
		}
	}

	env.RunApptainer(
		t,
		AsSubtest("LabelChecks"),
		WithProfile(profile),
		WithCommand("inspect"),
		WithArgs([]string{"--json", "--labels", imagePath}...),
		ExpectExit(0, verifyOutput),
	)
}

// DefinitionImageVerify checks for image correctness based off off supplied DefFileDetail
func DefinitionImageVerify(t *testing.T, cmdPath, imagePath string, dfd DefFileDetails) {
	if dfd.Help != nil {
		helpPath := filepath.Join(imagePath, `/.singularity.d/runscript.help`)
		if !fs.IsFile(helpPath) {
			t.Fatalf("unexpected failure: Script %v does not exist in container", helpPath)
		}

		if err := verifyHelp(t, helpPath, dfd.Help); err != nil {
			t.Fatalf("unexpected failure: help message: %v", err)
		}
	}

	if dfd.Env != nil {
		if err := verifyEnv(t, cmdPath, imagePath, dfd.Env, nil); err != nil {
			t.Fatalf("unexpected failure: Env in container is incorrect: %v", err)
		}
	}

	// verify %files section works correctly
	for _, p := range dfd.Files {
		var file string
		if p.Dst == "" {
			file = p.Src
		} else {
			file = p.Dst
		}

		if !fs.IsFile(filepath.Join(imagePath, file)) {
			t.Fatalf("unexpected failure: File %v does not exist in container", file)
		}

		if err := verifyFile(t, p.Src, filepath.Join(imagePath, file)); err != nil {
			t.Fatalf("unexpected failure: File %v: %v", file, err)
		}
	}

	if dfd.RunScript != nil {
		scriptPath := filepath.Join(imagePath, `/.singularity.d/runscript`)
		if !fs.IsFile(scriptPath) {
			t.Fatalf("unexpected failure: Script %v does not exist in container", scriptPath)
		}

		if err := verifyScript(t, scriptPath, dfd.RunScript); err != nil {
			t.Fatalf("unexpected failure: runscript: %v", err)
		}
	}

	if dfd.StartScript != nil {
		scriptPath := filepath.Join(imagePath, `/.singularity.d/startscript`)
		if !fs.IsFile(scriptPath) {
			t.Fatalf("unexpected failure: Script %v does not exist in container", scriptPath)
		}

		if err := verifyScript(t, scriptPath, dfd.StartScript); err != nil {
			t.Fatalf("unexpected failure: startscript: %v", err)
		}
	}

	if dfd.Test != nil {
		scriptPath := filepath.Join(imagePath, `/.singularity.d/test`)
		if !fs.IsFile(scriptPath) {
			t.Fatalf("unexpected failure: Script %v does not exist in container", scriptPath)
		}

		if err := verifyScript(t, scriptPath, dfd.Test); err != nil {
			t.Fatalf("unexpected failure: test script: %v", err)
		}
	}

	for _, file := range dfd.Pre {
		if !fs.IsFile(file) {
			t.Fatalf("unexpected failure: %%Pre generated file %v does not exist on host", file)
		}
		if err := os.Remove(file); err != nil {
			t.Fatalf("could not remove %s: %s", file, err)
		}
	}

	for _, file := range dfd.Setup {
		if !fs.IsFile(file) {
			t.Fatalf("unexpected failure: %%Setup generated file %v does not exist on host", file)
		}
		if err := os.Remove(file); err != nil {
			t.Fatalf("could not remove %s: %s", file, err)
		}
	}

	for _, file := range dfd.Post {
		if !fs.IsFile(filepath.Join(imagePath, file)) {
			t.Fatalf("unexpected failure: %%Post generated file %v does not exist in container", file)
		}
	}

	// Verify any apps
	for _, app := range dfd.Apps {
		// %apphelp
		if app.Help != nil {
			helpPath := filepath.Join(imagePath, `/scif/apps/`, app.Name, `/scif/runscript.help`)
			if !fs.IsFile(helpPath) {
				t.Fatalf("unexpected failure in app %v: Script %v does not exist in app", app.Name, helpPath)
			}

			if err := verifyHelp(t, helpPath, app.Help); err != nil {
				t.Fatalf("unexpected failure in app %v: app help message: %v", app.Name, err)
			}
		}

		// %appenv
		if app.Env != nil {
			if err := verifyEnv(t, cmdPath, imagePath, app.Env, []string{"--app", app.Name}); err != nil {
				t.Fatalf("unexpected failure in app %v: Env in app is incorrect: %v", app.Name, err)
			}
		}

		// %appfiles
		for _, p := range app.Files {
			var file string
			if p.Src == "" {
				file = p.Src
			} else {
				file = p.Dst
			}

			if !fs.IsFile(filepath.Join(imagePath, "/scif/apps/", app.Name, file)) {
				t.Fatalf("unexpected failure in app %v: File %v does not exist in app", app.Name, file)
			}

			if err := verifyFile(t, p.Src, filepath.Join(imagePath, "/scif/apps/", app.Name, file)); err != nil {
				t.Fatalf("unexpected failure in app %v: File %v: %v", app.Name, file, err)
			}
		}

		// %appInstall
		for _, file := range app.Install {
			if !fs.IsFile(filepath.Join(imagePath, "/scif/apps/", app.Name, file)) {
				t.Fatalf("unexpected failure in app %v: %%Install generated file %v does not exist in container", app.Name, file)
			}
		}

		// %appRun
		if app.Run != nil {
			scriptPath := filepath.Join(imagePath, "/scif/apps/", app.Name, "scif/runscript")
			if !fs.IsFile(scriptPath) {
				t.Fatalf("unexpected failure in app %v: Script %v does not exist in app", app.Name, scriptPath)
			}

			if err := verifyScript(t, scriptPath, app.Run); err != nil {
				t.Fatalf("unexpected failure in app %v: runscript: %v", app.Name, err)
			}
		}

		// %appTest
		if app.Test != nil {
			scriptPath := filepath.Join(imagePath, "/scif/apps/", app.Name, "scif/test")
			if !fs.IsFile(scriptPath) {
				t.Fatalf("unexpected failure in app %v: Script %v does not exist in app", app.Name, scriptPath)
			}

			if err := verifyScript(t, scriptPath, app.Test); err != nil {
				t.Fatalf("unexpected failure in app %v: test script: %v", app.Name, err)
			}
		}
	}
}

func verifyFile(t *testing.T, original, copy string) error {
	ofi, err := os.Stat(original)
	if err != nil {
		t.Fatalf("While getting file info: %v", err)
	}

	cfi, err := os.Stat(copy)
	if err != nil {
		t.Fatalf("While getting file info: %v", err)
	}

	if ofi.Size() != cfi.Size() {
		return fmt.Errorf("incorrect file sizes. Original: %v, Copy: %v", ofi.Size(), cfi.Size())
	}

	if ofi.Mode() != cfi.Mode() {
		return fmt.Errorf("incorrect file modes. Original: %v, Copy: %v", ofi.Mode(), cfi.Mode())
	}

	o, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("While reading file: %v", err)
	}

	c, err := os.ReadFile(copy)
	if err != nil {
		t.Fatalf("While reading file: %v", err)
	}

	if !bytes.Equal(o, c) {
		return fmt.Errorf("incorrect file content")
	}

	return nil
}

func verifyHelp(t *testing.T, fileName string, contents []string) error {
	fi, err := os.Stat(fileName)
	if err != nil {
		t.Fatalf("While getting file info: %v", err)
	}

	// do perm check
	if fi.Mode().Perm() != 0o644 {
		return fmt.Errorf("incorrect help script perms: %v", fi.Mode().Perm())
	}

	s, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("While reading file: %v", err)
	}

	helpScript := string(s)
	for _, c := range contents {
		if !strings.Contains(helpScript, c) {
			return fmt.Errorf("missing help script content")
		}
	}

	return nil
}

func verifyScript(t *testing.T, fileName string, contents []string) error {
	fi, err := os.Stat(fileName)
	if err != nil {
		t.Fatalf("While getting file info: %v", err)
	}

	// do perm check
	if fi.Mode().Perm() != 0o755 {
		return fmt.Errorf("incorrect script perms: %v", fi.Mode().Perm())
	}

	s, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("While reading file: %v", err)
	}

	script := string(s)
	for _, c := range contents {
		if !strings.Contains(script, c) {
			return fmt.Errorf("missing script content")
		}
	}

	return nil
}

func verifyEnv(t *testing.T, cmdPath, imagePath string, env []string, flags []string) error {
	args := []string{"exec"}
	if flags != nil {
		args = append(args, flags...)
	}
	args = append(args, imagePath, "env")

	cmd := exec.Command(cmdPath, args...)
	res := cmd.Run(t)

	if res.Error != nil {
		t.Fatalf("Error running command.\n%s", res)
	}

	out := res.Stdout()

	for _, e := range env {
		if !strings.Contains(out, e) {
			return fmt.Errorf("environment is missing: %v", e)
		}
	}

	return nil
}
