// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package imgbuild

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/google/uuid"
)

// This test will build an image from a multi-stage definition
// file, the first stage compile a bad NSS library containing
// a constructor forcing program to exit with code 255 when loaded,
// the second stage will copy the bad NSS library in its root filesystem
// to check that the post section executed by the build engine doesn't
// load the bad NSS library from container image.
// Most if not all NSS services point to the bad NSS library in
// order to catch all the potential calls which could occur from
// Go code inside the build engine, apptainer engine is also tested.
func (c imgBuildTests) issue4203(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_4203.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_4203.def"),
		e2e.PostRun(func(t *testing.T) {
			defer os.Remove(image)

			if t.Failed() {
				return
			}

			// also execute the image to check that apptainer
			// engine doesn't try to load a NSS library from
			// container image
			c.env.RunApptainer(
				t,
				e2e.WithProfile(e2e.UserProfile),
				e2e.WithCommand("exec"),
				e2e.WithArgs(image, "true"),
				e2e.ExpectExit(0),
			)
		}),
		e2e.ExpectExit(0),
	)
}

// issue4407 checks that it's possible to build a sandbox image when the
// destination directory contains a trailing slash and when it doesn't.
func (c *imgBuildTests) issue4407(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	sandboxDir := func() string {
		name, err := os.MkdirTemp(c.env.TestDir, "sandbox.")
		if err != nil {
			log.Fatalf("failed to create temporary directory for sandbox: %v", err)
		}

		if err := os.Chmod(name, 0o755); err != nil {
			log.Fatalf("failed to chmod temporary directory for sandbox: %v", err)
		}

		return name
	}

	tc := map[string]string{
		"with slash":    sandboxDir() + "/",
		"without slash": sandboxDir(),
	}

	for name, imagePath := range tc {
		args := []string{
			"--force",
			"--sandbox",
			imagePath,
			c.env.ImagePath,
		}

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.PostRun(func(t *testing.T) {
				if t.Failed() {
					return
				}

				defer os.RemoveAll(imagePath)

				c.env.ImageVerify(t, imagePath, e2e.RootProfile)
			}),
			e2e.ExpectExit(0),
		)
	}
}

func (c *imgBuildTests) issue4583(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_4583.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_4583.def"),
		e2e.PostRun(func(t *testing.T) {
			defer os.Remove(image)

			if t.Failed() {
				return
			}
		}),
		e2e.ExpectExit(0),
	)
}

func (c imgBuildTests) issue4837(t *testing.T) {
	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	sandboxName := id.String()

	u := e2e.FakerootProfile.HostUser(t)

	def, err := filepath.Abs("testdata/Apptainer")
	if err != nil {
		t.Fatalf("failed to retrieve absolute path for testdata/Apptainer: %s", err)
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithDir(u.Dir),
		e2e.WithCommand("build"),
		e2e.WithArgs("--sandbox", sandboxName, def),
		e2e.PostRun(func(t *testing.T) {
			if !t.Failed() {
				os.RemoveAll(filepath.Join(u.Dir, sandboxName))
			}
		}),
		e2e.ExpectExit(0),
	)
}

// Test %post -c section parameter is correctly handled. We use `-c /bin/busybox
// sh` for this test, and can observe the `/proc/$$/cmdline` to check that was
// used to invoke the post script.
func (c *imgBuildTests) issue4967(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_4967.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_4967.def"),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(image)
		}),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "/bin/busybox sh /.post.script"),
		),
	)
}

// The image contains symlinks /etc/resolv.conf and /etc/hosts
// pointing to nowhere, build should pass but with warnings.
func (c *imgBuildTests) issue4969(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_4969.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_4969.def"),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(image)
		}),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "TEST OK"),
		),
	)
}

func (c *imgBuildTests) issue5166(t *testing.T) {
	// create a directory that we don't to be overwritten by mistakes
	sensibleDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "sensible-dir-", "")

	secret := filepath.Join(sensibleDir, "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatalf("could not create %s: %s", secret, err)
	}

	image := filepath.Join(c.env.TestDir, "issue_4969.sandbox")

	e2e.EnsureImage(t, c.env)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--sandbox", image, c.env.ImagePath),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--sandbox", sensibleDir, c.env.ImagePath),
		e2e.ExpectExit(
			255,
			e2e.ExpectError(
				e2e.ContainMatch,
				"use --force if you want to overwrite it",
			),
		),
	)

	// finally force overwrite
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sensibleDir, c.env.ImagePath),
		e2e.PostRun(func(t *testing.T) {
			if !t.Failed() {
				cleanup(t)
			}
		}),
		e2e.ExpectExit(0),
	)
}

// SCIF apps must build in order - build a recipe where the second app depends
// on things created in the first apps's appinstall section
func (c *imgBuildTests) issue4820(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_4820.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_4820.def"),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(image)
		}),
		e2e.ExpectExit(
			0,
		),
	)
}

// When running a %test section under fakeroot we must recognize the engine
// is running under a user namespace and avoid overlay.
func (c *imgBuildTests) issue5315(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_5315.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_5315.def"),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(image)
		}),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "TEST OK"),
		),
	)
}

// This test will attempt to build an image by passing an empty string as
// the build destination. This should fail.
func (c *imgBuildTests) issue5435(t *testing.T) {
	// create a directory that we don't care about
	cwd, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "throwaway-dir-", "")
	defer func(t *testing.T) {
		if !t.Failed() {
			cleanup(t)
		}
	}(t)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("", ""),
		e2e.WithDir(cwd),
		e2e.PostRun(func(t *testing.T) {
			exists, err := fs.PathExists(cwd)
			if err != nil {
				t.Fatalf("failed to check cwd: %v", err)
			}

			if !exists {
				t.Fatalf("cwd no longer exists")
			}

			if !fs.IsDir(cwd) {
				t.Fatalf("cwd overwritten")
			}
		}),
		e2e.ExpectExit(255),
	)
}

// This test will yum reinstall the 'setup' package in a centos 7 container during %post.
// On a CentOS/RHEL/Fedora host this yum reinstall errors unless the bound in /etc/hosts in the build is modified from
// the package default, so that yum does not attempt to remove->replace it (which is not possible as it is bound in).
// See the workaround in build.createStageFile
func (c *imgBuildTests) issue5250(t *testing.T) {
	image := filepath.Join(c.env.TestDir, "issue_5250.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, "testdata/regressions/issue_5250.def"),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(image)
		}),
		e2e.ExpectExit(
			0,
		),
	)
}

// Check that unsquashfs (SIF -> sandbox) works on a tmpfs, that will not support
// user xattrs. Our home dir in the e2e test is a tmpfs bound over our real home dir
// so we can use that.
func (c *imgBuildTests) issue5668(t *testing.T) {
	e2e.EnsureImage(t, c.env)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Could not get home dir: %v", err)
	}
	sbDir, sbCleanup := e2e.MakeTempDir(t, home, "issue-5668-", "")
	defer e2e.Privileged(sbCleanup)(t)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sbDir, c.env.ImagePath),
		e2e.ExpectExit(0),
	)
}

// Check that unsquashfs (for version >= 4.4) works for non root users when image contains
// pseudo devices in /dev.
func (c *imgBuildTests) issue5690(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	sandbox, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "issue-5690-", "")
	defer e2e.Privileged(cleanup)(t)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sandbox, c.env.ImagePath),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sandbox, c.env.ImagePath),
		e2e.ExpectExit(0),
	)
}

func (c *imgBuildTests) issue3848(t *testing.T) {
	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "issue-3848-", "")
	defer cleanup(t)

	f, err := os.CreateTemp(tmpDir, "test-def-")
	if err != nil {
		t.Fatalf("failed to open temp file: %v", err)
	}
	defer f.Close()

	tmpfile, err := e2e.WriteTempFile(tmpDir, "test-file-", testFileContent)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile) // clean up

	d := struct {
		From string
		File string
	}{
		From: busyboxSIF,
		File: tmpfile,
	}

	defTmpl := `Bootstrap: localimage
From: {{ .From }}

%files
	{{ .File }}

%files #  # from test
	{{ .File }}

%files #   #comment
	{{ .File }}
`

	tmpl, err := template.New("test").Parse(defTmpl)
	if err != nil {
		t.Fatalf("while parsing pattern: %v", err)
	}

	if err := tmpl.Execute(f, d); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	image := path.Join(tmpDir, "image.sif")
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(image, f.Name()),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(image)
		}),
		e2e.ExpectExit(0),
	)
}
