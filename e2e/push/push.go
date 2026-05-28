// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package push tests only test the oras transport (and a invalid transport) against a local registry
package push

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

type ctx struct {
	env e2e.TestEnv
}

func (c ctx) testInvalidTransport(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name       string
		uri        string
		expectOp   e2e.ApptainerCmdResultOp
		expectExit int
	}{
		{
			name:       "push invalid transport",
			uri:        "nothing://bar/foo/foobar:latest",
			expectOp:   e2e.ExpectError(e2e.ContainMatch, "Unsupported transport type: nothing"),
			expectExit: 255,
		},
	}

	for _, tt := range tests {
		args := []string{c.env.ImagePath, tt.uri}

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("push"),
			e2e.WithArgs(args...),
			e2e.ExpectExit(tt.expectExit, tt.expectOp),
		)
	}
}

func (c ctx) testPushCmd(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	// setup file and dir to use as invalid sources
	orasInvalidDir, err := os.MkdirTemp(c.env.TestDir, "oras_push_dir-")
	if err != nil {
		err = errors.Wrap(err, "creating oras temporary directory")
		t.Fatalf("unable to create src dir for push tests: %+v", err)
	}

	orasInvalidFile, err := e2e.WriteTempFile(orasInvalidDir, "oras_invalid_image-", "Invalid Image Contents")
	if err != nil {
		err = errors.Wrap(err, "creating oras temporary file")
		t.Fatalf("unable to create src file for push tests: %+v", err)
	}

	tests := []struct {
		desc             string   // case description
		dstURI           string   // destination URI for image
		imagePath        string   // src image path
		expectedExitCode int      // expected exit code for the test
		noHTTPS          bool     // --no-https/--nohttps flag
		description      string   // --description
		annotations      []string // --annotation
		expectedManifest string   // expected partial manifest content
	}{
		{
			desc:             "non existent image",
			imagePath:        filepath.Join(orasInvalidDir, "not_an_existing_file.sif"),
			dstURI:           fmt.Sprintf("oras://%s/non_existent:test", c.env.TestRegistry),
			expectedExitCode: 255,
		},
		{
			desc:             "non SIF file",
			imagePath:        orasInvalidFile,
			dstURI:           fmt.Sprintf("oras://%s/non_sif:test", c.env.TestRegistry),
			expectedExitCode: 255,
		},
		{
			desc:             "directory",
			imagePath:        orasInvalidDir,
			dstURI:           fmt.Sprintf("oras://%s/directory:test", c.env.TestRegistry),
			expectedExitCode: 255,
		},
		// this will succeed, because go-containerregistry will automatically switch to insecure mode if it's locallhost
		{
			desc:             "standard SIF push",
			imagePath:        c.env.ImagePath,
			dstURI:           fmt.Sprintf("oras://%s/standard_sif:test", c.env.InsecureRegistry),
			expectedExitCode: 0,
		},
		{
			desc:             "standard SIF push with --no-https/--nohttps",
			imagePath:        c.env.ImagePath,
			dstURI:           fmt.Sprintf("oras://%s/standard_sif:test_nohttps", c.env.InsecureRegistry),
			noHTTPS:          true,
			expectedExitCode: 0,
		},
		{
			desc:             "standard SIF push with --description",
			imagePath:        c.env.ImagePath,
			dstURI:           fmt.Sprintf("oras://%s/standard_sif:test_description", c.env.InsecureRegistry),
			description:      "description",
			expectedExitCode: 0,
			expectedManifest: `"org.opencontainers.image.description"`,
		},
		{
			desc:             "standard SIF push with --annotation",
			imagePath:        c.env.ImagePath,
			dstURI:           fmt.Sprintf("oras://%s/standard_sif:test_annotation", c.env.InsecureRegistry),
			annotations:      []string{"foo=x", "bar=y"},
			expectedExitCode: 0,
			expectedManifest: `"bar":"y","foo":"x"`, // alphabetical order
		},
	}

	for _, tt := range tests {
		tmpdir, err := os.MkdirTemp(c.env.TestDir, "pull_test.")
		if err != nil {
			t.Fatalf("Failed to create temporary directory for pull test: %+v", err)
		}
		defer os.RemoveAll(tmpdir)

		// We create the list of arguments using a string instead of a slice of
		// strings because using slices of strings most of the type ends up adding
		// an empty elements to the list when passing it to the command, which
		// will create a failure.
		args := tt.dstURI
		if tt.imagePath != "" {
			args = tt.imagePath + " " + args
		}

		if tt.noHTTPS {
			args = "--no-https" + " " + args
		}

		if tt.description != "" {
			args = "--description=" + tt.description + " " + args
		}
		for _, annotation := range tt.annotations {
			args = "--annotation=" + annotation + " " + args
		}

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.desc),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("push"),
			e2e.WithArgs(strings.Split(args, " ")...),
			e2e.ExpectExit(tt.expectedExitCode),
		)

		if tt.expectedManifest != "" {
			ref := strings.TrimPrefix(tt.dstURI, "oras://")
			ir, err := name.ParseReference(ref)
			if err != nil {
				t.Fatalf("error parsing image: %+v", err)
			}
			image, err := remote.Image(ir)
			if err != nil {
				t.Fatalf("error getting image: %+v", err)
			}
			manifest, err := image.RawManifest()
			if err != nil {
				t.Fatalf("error getting manifest: %+v", err)
			}
			if !strings.Contains(string(manifest), tt.expectedManifest) {
				t.Errorf("did not find %s", tt.expectedManifest)
			}
		}
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"invalid transport": c.testInvalidTransport,
		"oras":              c.testPushCmd,
	}
}
