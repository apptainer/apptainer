// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package pull

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
)

// If a remote is set to a different endpoint we should be able to pull
// with `--library https://library.production.sycloud.io` from the default Sylabs cloud library.
func (c ctx) issue5808(t *testing.T) {
	testEndpoint := "issue5808"
	testEndpointURI := "https://cloud.sycloud.io"
	defaultLibraryURI := "https://library.production.sycloud.io"
	testImage := "library://sylabs/tests/signed:1.0.0"

	pullDir, cleanup := e2e.MakeTempDir(t, "", "issue5808", "")
	defer cleanup(t)

	// Add another endpoint
	argv := []string{"add", "--no-login", testEndpoint, testEndpointURI}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("remote add"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("remote"),
		e2e.WithArgs(argv...),
		e2e.ExpectExit(0),
	)
	// Remove test remote when we are done here
	defer func(t *testing.T) {
		argv := []string{"remove", testEndpoint}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest("remote remove"),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}(t)

	// Set as default
	argv = []string{"use", testEndpoint}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("remote use"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("remote"),
		e2e.WithArgs(argv...),
		e2e.ExpectExit(0),
	)

	// Pull a library image
	dest := path.Join(pullDir, "alpine.sif")
	argv = []string{"--arch", "amd64", "--library", defaultLibraryURI, dest, testImage}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs(argv...),
		e2e.ExpectExit(0),
	)
}

// Must be able to pull from an http(s) source that doesn't set content-length correctly
func (c ctx) issueSylabs1087(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "issue-1087-", "")
	defer cleanup(t)
	pullPath := filepath.Join(tmpDir, "test.sif")

	// Start an http server that serves some output without a content-length
	data := "DATADATADATADATA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, data)
	}))
	defer srv.Close()

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs(pullPath, srv.URL),
		e2e.ExpectExit(0),
	)

	content, err := os.ReadFile(pullPath)
	if err != nil {
		t.Error(err)
	}
	if string(content) != data {
		t.Errorf("Content of file not correct. Expected %s, got %s", data, content)
	}
}

// APPTAINER_PULLDIR must not break pulling when the destination is an
// absolute path, e.g. via `pull --name /tmp/test.sif`. Before the fix,
// the absolute path was joined below pullDir, producing an invalid
// destination such as /tmp/pull/tmp/test.sif.
func (c ctx) pullDirWithAbsolutePath(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "issue-3672-", "")
	defer cleanup(t)

	pullDir := filepath.Join(tmpDir, "pull")
	if err := os.Mkdir(pullDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Serve the local test image over http(s) so that pull performs a
	// download and then copies to the requested destination.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, c.env.ImagePath)
	}))
	defer srv.Close()

	// Absolute destination path, as with `--name`.
	dest := filepath.Join(tmpDir, "test.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithEnv(append(os.Environ(), "APPTAINER_PULLDIR="+pullDir)),
		e2e.WithArgs("--name", dest, srv.URL),
		e2e.ExpectExit(0),
	)

	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected image at %s: %v", dest, err)
	}
}
