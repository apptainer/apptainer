// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// The E2E PULL group tests image pulls of SIF format images (library, oras
// sources). Docker / OCI image pull is tested as part of the DOCKER E2E group.

package pull

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/client/oras"
	syoras "github.com/apptainer/apptainer/internal/pkg/client/oras"
	"github.com/apptainer/apptainer/internal/pkg/util/uri"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/sys/unix"
)

type ctx struct {
	env e2e.TestEnv
}

type testStruct struct {
	desc             string // case description
	srcURI           string // source URI for image
	library          string // use specific library, XXX(mem): not tested yet
	arch             string // architecture to force, if any
	force            bool   // pass --force
	createDst        bool   // create destination file before pull
	unauthenticated  bool   // pass --allow-unauthenticated
	setImagePath     bool   // pass destination path
	setPullDir       bool   // pass --dir
	expectedExitCode int
	workDir          string
	pullDir          string
	imagePath        string
	expectedImage    string
	envVars          []string
	disabled         bool
	noHTTPS          bool
}

func (c *ctx) imagePull(t *testing.T, tt testStruct) {
	// Use a one-time cache directory specific to this pull. This ensures we are always
	// testing an entire pull operation, performing the download into an empty cache.
	cacheDir, cleanup := e2e.MakeCacheDir(t, "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	c.env.UnprivCacheDir = cacheDir

	// We use a string rather than a slice of strings to avoid having an empty
	// element in the slice, which would cause the command to fail, without
	// over-complicating the code.
	argv := ""

	if tt.arch != "" {
		argv += "--arch " + tt.arch + " "
	}

	if tt.force {
		argv += "--force "
	}

	if tt.unauthenticated {
		argv += "--allow-unauthenticated "
	}

	if tt.pullDir != "" {
		argv += "--dir " + tt.pullDir + " "
	}

	if tt.library != "" {
		argv += "--library " + tt.library + " "
	}

	if tt.imagePath != "" {
		argv += tt.imagePath + " "
	}

	if tt.noHTTPS {
		argv += "--no-https "
	}

	if tt.workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("unable to get working directory: %s", err)
		}
		tt.workDir = wd
	}

	argv += tt.srcURI

	c.env.RunApptainer(
		t,
		e2e.AsSubtest(tt.desc),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithEnv(tt.envVars),
		e2e.WithDir(tt.workDir),
		e2e.WithCommand("pull"),
		e2e.WithArgs(strings.Split(argv, " ")...),
		e2e.ExpectExit(tt.expectedExitCode))

	checkPullResult(t, tt)
}

func getImageNameFromURI(imgURI string) string {
	// XXX(mem): this function should be part of the code, not the test
	switch transport, ref := uri.Split(imgURI); {
	case ref == "":
		return "" // Invalid URI

	case transport == "":
		imgURI = "oras://" + imgURI
	}

	return uri.GetName(imgURI)
}

func (c *ctx) setup(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	orasInvalidFile, err := e2e.WriteTempFile(c.env.TestDir, "oras_invalid_image-", "Invalid Image Contents")
	if err != nil {
		t.Fatalf("unable to create src file for push tests: %v", err)
	}

	// prep local registry with oras generated artifacts
	// Note: the image name prevents collisions by using a package specific name
	// as the registry is shared between different test packages
	orasImages := []struct {
		srcPath        string
		uri            string
		layerMediaType string
	}{
		{
			srcPath:        c.env.ImagePath,
			uri:            fmt.Sprintf("%s/pull_test_sif:latest", c.env.TestRegistry),
			layerMediaType: syoras.SifLayerMediaTypeV1,
		},
		{
			srcPath:        c.env.ImagePath,
			uri:            fmt.Sprintf("%s/pull_test_sif_mediatypeproto:latest", c.env.TestRegistry),
			layerMediaType: syoras.SifLayerMediaTypeProto,
		},
		{
			srcPath:        orasInvalidFile,
			uri:            fmt.Sprintf("%s/pull_test_invalid_file:latest", c.env.TestRegistry),
			layerMediaType: syoras.SifLayerMediaTypeV1,
		},
	}

	for _, i := range orasImages {
		err = orasPushNoCheck(i.srcPath, i.uri, i.layerMediaType)
		if err != nil {
			t.Fatalf("while prepping registry for oras tests: %v", err)
		}
	}
}

func (c ctx) testPullCmd(t *testing.T) {
	tests := []testStruct{
		{
			desc:             "non existent image",
			srcURI:           "oras://image/does/not:exist",
			expectedExitCode: 255,
		},

		// --allow-unauthenticated tests
		{
			desc:             "unsigned image allow unauthenticated",
			srcURI:           "oras://ghcr.io/apptainer/alpine:3.15.0",
			unauthenticated:  true,
			expectedExitCode: 0,
		},

		// --force tests
		{
			desc:             "force existing file",
			srcURI:           "oras://ghcr.io/apptainer/alpine:3.15.0",
			force:            true,
			createDst:        true,
			unauthenticated:  true,
			expectedExitCode: 0,
		},
		{
			desc:             "force non-existing file",
			srcURI:           "oras://ghcr.io/apptainer/alpine:3.15.0",
			force:            true,
			createDst:        false,
			unauthenticated:  true,
			expectedExitCode: 0,
		},
		{
			// --force should not have an effect on --allow-unauthenticated=false
			desc:             "unsigned image force require authenticated",
			srcURI:           "oras://ghcr.io/apptainer/alpine:3.15.0",
			force:            true,
			unauthenticated:  false,
			expectedExitCode: 0,
		},

		// test version specifications
		{
			desc:             "image with specific hash",
			srcURI:           "oras://ghcr.io/apptainer/alpine@sha256:aef2a1baf177ee2e6f21da40bdb7025f58466d39116507837f74c2ab4abf5606",
			arch:             "amd64",
			unauthenticated:  true,
			expectedExitCode: 0,
		},
		{
			desc:             "latest tag",
			srcURI:           "oras://ghcr.io/apptainer/alpine:latest",
			unauthenticated:  true,
			expectedExitCode: 0,
		},

		// --dir tests
		{
			desc:             "dir no image path",
			srcURI:           "oras://ghcr.io/apptainer/alpine:3.15.0",
			unauthenticated:  true,
			setPullDir:       true,
			setImagePath:     false,
			expectedExitCode: 0,
		},
		{
			// XXX(mem): this specific test is passing both --path and an image path to
			// apptainer pull. The current behavior is that the code is joining both paths and
			// failing to find the image in the expected location indicated by image path
			// because image path is absolute, so after joining /tmp/a/b/c and
			// /tmp/a/b/image.sif, the code expects to find /tmp/a/b/c/tmp/a/b/image.sif. Since
			// the directory /tmp/a/b/c/tmp/a/b does not exist, it fails to create the file
			// image.sif in there.
			desc:             "dir image path",
			srcURI:           "oras://ghcr.io/apptainer/alpine:3.15.0",
			unauthenticated:  true,
			setPullDir:       true,
			setImagePath:     true,
			expectedExitCode: 255,
		},

		// transport tests
		{
			desc:             "bare image name",
			srcURI:           "alpine:3.15.0",
			force:            true,
			unauthenticated:  true,
			expectedExitCode: 0,
			disabled:         true,
		},

		{
			desc:             "image from docker",
			srcURI:           "docker://alpine:3.8",
			force:            true,
			unauthenticated:  false,
			expectedExitCode: 0,
		},
		// TODO(mem): re-enable this; disabled while shub is down
		{
			desc:             "image from shub",
			srcURI:           "shub://GodloveD/busybox",
			force:            true,
			unauthenticated:  false,
			expectedExitCode: 0,
			disabled:         true,
		},
		// Finalized v1 layer mediaType (3.7 and onward)
		{
			desc:             "oras transport for SIF from registry",
			srcURI:           fmt.Sprintf("oras://%s/pull_test_sif:latest", c.env.TestRegistry),
			force:            true,
			unauthenticated:  false,
			expectedExitCode: 0,
		},
		// Original/prototype layer mediaType (<3.7)
		{
			desc:             "oras transport for SIF from registry (SifLayerMediaTypeProto)",
			srcURI:           fmt.Sprintf("oras://%s/pull_test_sif_mediatypeproto:latest", c.env.TestRegistry),
			force:            true,
			unauthenticated:  false,
			expectedExitCode: 0,
		},

		// pulling of invalid images with oras
		{
			desc:             "oras pull of non SIF file",
			srcURI:           fmt.Sprintf("oras://%s/pull_test_:latest", c.env.TestRegistry),
			force:            true,
			expectedExitCode: 255,
		},
		{
			desc:             "oras pull of packed dir",
			srcURI:           fmt.Sprintf("oras://%s/pull_test_invalid_file:latest", c.env.TestRegistry),
			force:            true,
			expectedExitCode: 255,
		},

		// pulling with library URI argument
		{
			desc:             "bad library URI",
			srcURI:           "oras://ghcr.io/apptainer/bad/busybox:1.31.1",
			library:          "https://bad-library.production.sycloud.io",
			expectedExitCode: 255,
		},
		{
			desc:             "default library URI",
			srcURI:           "oras://ghcr.io/apptainer/busybox:1.31.1",
			library:          "https://library.production.sycloud.io",
			force:            true,
			expectedExitCode: 0,
		},

		// pulling with --no-https flag
		{
			desc:             "oras pull of SIF file with --no-https flag should succeed",
			srcURI:           fmt.Sprintf("oras://%s/pull_test_sif:latest", c.env.InsecureRegistry),
			unauthenticated:  true,
			force:            true,
			noHTTPS:          true,
			expectedExitCode: 0,
		},

		// pulling without --no-https flag
		{
			desc:             "oras pull of SIF file should success because go-containerregistry will automatically switch to insecure mode for localhost",
			srcURI:           fmt.Sprintf("oras://%s/pull_test_sif:latest", c.env.InsecureRegistry),
			unauthenticated:  true,
			force:            true,
			expectedExitCode: 0,
		},
	}

	for _, tt := range tests {
		if tt.disabled {
			continue
		}
		t.Run(tt.desc, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp(c.env.TestDir, "pull_test.")
			if err != nil {
				t.Fatalf("Failed to create temporary directory for pull test: %+v", err)
			}
			t.Cleanup(func() {
				if !t.Failed() {
					os.RemoveAll(tmpdir)
				}
			})

			if tt.setPullDir {
				tt.pullDir, err = os.MkdirTemp(tmpdir, "pull_dir.")
				if err != nil {
					t.Fatalf("Failed to create temporary directory for pull dir: %+v", err)
				}
			}

			if tt.setImagePath {
				tt.imagePath = filepath.Join(tmpdir, "image.sif")
				tt.expectedImage = tt.imagePath
			} else {
				// No explicit image path specified. Will use temp dir as working directory,
				// so we pull into a clean location.
				tt.workDir = tmpdir
				imageName := getImageNameFromURI(tt.srcURI)
				tt.expectedImage = filepath.Join(tmpdir, imageName)

				// if there's a pullDir, that's where we expect to find the image
				if tt.pullDir != "" {
					tt.expectedImage = filepath.Join(tt.pullDir, imageName)
				}

			}

			// In order to actually test force, there must already be a file present in
			// the expected location
			if tt.createDst {
				fh, err := os.Create(tt.expectedImage)
				if err != nil {
					t.Fatalf("failed to create file %q: %+v\n", tt.expectedImage, err)
				}
				fh.Close()
			}

			c.imagePull(t, tt)
		})
	}
}

func checkPullResult(t *testing.T, tt testStruct) {
	if tt.expectedExitCode == 0 {
		_, err := os.Stat(tt.expectedImage)
		switch err {
		case nil:
			// PASS
			return

		case os.ErrNotExist:
			// FAIL
			t.Errorf("expecting image at %q, not found: %+v\n", tt.expectedImage, err)

		default:
			// FAIL
			t.Errorf("unable to stat image at %q: %+v\n", tt.expectedImage, err)
		}

		// XXX(mem): This is running a bunch of commands in the downloaded
		// images. Do we really want this here? If yes, we need to have a
		// way to do this in a generic fashion, as it's going to be shared
		// with build as well.

		// imageVerify(t, tt.imagePath, false)
	}
}

// this is a version of the oras push functionality that does not check that given the
// file is a valid SIF, this allows us to push arbitrary objects to the local registry
// to test the pull validation
// We can also set the layer mediaType - so we can push images with older media types
// to verify that they can still be pulled.
func orasPushNoCheck(path, ref, layerMediaType string) error {
	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")

	// Get reference to image in the remote
	ir, err := name.ParseReference(ref,
		name.WithDefaultTag(name.DefaultTag),
		name.WithDefaultRegistry(name.DefaultRegistry),
	)
	if err != nil {
		return err
	}

	im, err := oras.NewImageFromSIF(path, types.MediaType(layerMediaType))
	if err != nil {
		return err
	}

	return remote.Write(ir, im, remote.WithUserAgent("singularity e2e-test"))
}

func (c ctx) testPullDisableCacheCmd(t *testing.T) {
	cacheDir, err := os.MkdirTemp("", "e2e-imgcache-")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			err := os.RemoveAll(cacheDir)
			if err != nil {
				t.Fatalf("failed to delete temporary directory %s: %s", cacheDir, err)
			}
		}
	})

	c.env.UnprivCacheDir = cacheDir

	disableCacheTests := []struct {
		name      string
		imagePath string
		imageSrc  string
	}{
		{
			name:      "library",
			imagePath: filepath.Join(c.env.TestDir, "library.sif"),
			imageSrc:  "oras://ghcr.io/apptainer/alpine:latest",
		},
		{
			name:      "oras",
			imagePath: filepath.Join(c.env.TestDir, "oras.sif"),
			imageSrc:  fmt.Sprintf("oras://%s/pull_test_sif:latest", c.env.TestRegistry),
		},
	}

	for _, tt := range disableCacheTests {
		cmdArgs := []string{"--disable-cache", tt.imagePath, tt.imageSrc}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("pull"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(0),
			e2e.PostRun(func(t *testing.T) {
				// Cache entry must not have been created
				cacheEntryPath := filepath.Join(cacheDir, "cache")
				if _, err := os.Stat(cacheEntryPath); !os.IsNotExist(err) {
					t.Errorf("cache created while disabled (%s exists)", cacheEntryPath)
				}
				// We also need to check the image pulled is in the correct place!
				// Issue #5628s
				_, err := os.Stat(tt.imagePath)
				if os.IsNotExist(err) {
					t.Errorf("image does not exist at %s", tt.imagePath)
				}
			}),
		)
	}
}

// testPullUmask will run some pull tests with different umasks, and
// ensure the output file has the correct permissions.
func (c ctx) testPullUmask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, c.env.ImagePath)
	}))
	defer srv.Close()

	umask22Image := "0022-umask-pull"
	umask77Image := "0077-umask-pull"
	umask27Image := "0027-umask-pull"

	umaskTests := []struct {
		name       string
		imagePath  string
		umask      int
		expectPerm uint32
		force      bool
	}{
		{
			name:       "0022 umask pull",
			imagePath:  filepath.Join(c.env.TestDir, umask22Image),
			umask:      0o022,
			expectPerm: 0o755,
		},
		{
			name:       "0077 umask pull",
			imagePath:  filepath.Join(c.env.TestDir, umask77Image),
			umask:      0o077,
			expectPerm: 0o700,
		},
		{
			name:       "0027 umask pull",
			imagePath:  filepath.Join(c.env.TestDir, umask27Image),
			umask:      0o027,
			expectPerm: 0o750,
		},

		// With the force flag, and override the image. The permission will
		// reset to 0666 after every test.
		{
			name:       "0022 umask pull override",
			imagePath:  filepath.Join(c.env.TestDir, umask22Image),
			umask:      0o022,
			expectPerm: 0o755,
			force:      true,
		},
		{
			name:       "0077 umask pull override",
			imagePath:  filepath.Join(c.env.TestDir, umask77Image),
			umask:      0o077,
			expectPerm: 0o700,
			force:      true,
		},
		{
			name:       "0027 umask pull override",
			imagePath:  filepath.Join(c.env.TestDir, umask27Image),
			umask:      0o027,
			expectPerm: 0o750,
			force:      true,
		},
	}

	// Helper function to get the file mode for a file.
	getFilePerm := func(t *testing.T, path string) uint32 {
		finfo, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed while getting file permission: %s", err)
		}
		return uint32(finfo.Mode().Perm())
	}

	// Set a common umask, then reset it back later.
	oldUmask := unix.Umask(0o022)
	defer unix.Umask(oldUmask)

	// TODO: should also check the cache umask.
	for _, tc := range umaskTests {
		var cmdArgs []string
		if tc.force {
			cmdArgs = append(cmdArgs, "--force")
		}
		cmdArgs = append(cmdArgs, tc.imagePath, srv.URL)

		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.PreRun(func(t *testing.T) {
				// Reset the file permission after every pull.
				err := os.Chmod(tc.imagePath, 0o666)
				if !os.IsNotExist(err) && err != nil {
					t.Fatalf("failed chmod-ing file: %s", err)
				}

				// Set the test umask.
				unix.Umask(tc.umask)
			}),
			e2e.PostRun(func(t *testing.T) {
				// Check the file permission.
				permOut := getFilePerm(t, tc.imagePath)
				if tc.expectPerm != permOut {
					t.Fatalf("Unexpected failure: expecting file perm: %o, got: %o", tc.expectPerm, permOut)
				}
			}),
			e2e.WithCommand("pull"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(0),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		// Run pull tests sequentially among themselves, as they perform a lot
		// of un-cached pulls which could otherwise lead to hitting rate limits.
		"ordered": func(t *testing.T) {
			// Setup a test registry to pull from (for oras).
			c.setup(t)
			t.Run("pull", c.testPullCmd)
			t.Run("pullDisableCache", c.testPullDisableCacheCmd)
			t.Run("concurrencyConfig", c.testConcurrencyConfig)
			t.Run("concurrentPulls", c.testConcurrentPulls)
		},
		"issueSylabs1087": c.issueSylabs1087,
		// Manipulates umask for the process, so must be run alone to avoid
		// causing permission issues for other tests.
		"pullUmaskCheck": np(c.testPullUmask),
		// Regressions
		// Manipulates remotes, so must run alone
		"issue5808": np(c.issue5808),
	}
}
