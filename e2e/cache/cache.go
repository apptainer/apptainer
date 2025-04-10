// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
)

type cacheTests struct {
	env e2e.TestEnv
}

const imgName = "alpine_latest.sif"

func prepTest(t *testing.T, testEnv e2e.TestEnv, testName string, cacheParentDir string, imagePath string) (imageURL string, cleanup func()) {
	// We will pull images from a temporary http server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, testEnv.ImagePath)
	}))
	cleanup = srv.Close

	// If the test imageFile is already present check it's not also in the cache
	// at the start of our test - we expect to pull it again and then see it
	// appear in the cache.
	if fs.IsFile(imagePath) {
		ensureNotCached(t, testName, srv.URL, cacheParentDir)
	}

	testEnv.UnprivCacheDir = cacheParentDir
	testEnv.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", imagePath, srv.URL}...),
		e2e.ExpectExit(0),
	)

	ensureCached(t, testName, srv.URL, cacheParentDir)
	return srv.URL, cleanup
}

func (c cacheTests) testNoninteractiveCacheCmds(t *testing.T) {
	tests := []struct {
		name               string
		options            []string
		needImage          bool
		expectedEmptyCache bool
		expectedOutput     string
		exit               int
	}{
		{
			name:               "clean force",
			options:            []string{"clean", "--force"},
			expectedOutput:     "",
			needImage:          true,
			expectedEmptyCache: true,
			exit:               0,
		},
		{
			name:               "clean force days beyond age",
			options:            []string{"clean", "--force", "--days", "30"},
			expectedOutput:     "",
			needImage:          true,
			expectedEmptyCache: false,
			exit:               0,
		},
		{
			name:               "clean force days within age",
			options:            []string{"clean", "--force", "--days", "0"},
			expectedOutput:     "",
			needImage:          true,
			expectedEmptyCache: true,
			exit:               0,
		},
		{
			name:           "clean help",
			options:        []string{"clean", "--help"},
			expectedOutput: "Clean your local Apptainer cache",
			needImage:      false,
			exit:           0,
		},
		{
			name:           "list help",
			options:        []string{"list", "--help"},
			expectedOutput: "List your local Apptainer cache",
			needImage:      false,
			exit:           0,
		},
		{
			name:               "list type",
			options:            []string{"list", "--type", "net"},
			needImage:          true,
			expectedOutput:     "There are 1 container file",
			expectedEmptyCache: false,
			exit:               0,
		},
		{
			name:               "list verbose",
			needImage:          true,
			options:            []string{"list", "--verbose"},
			expectedOutput:     "NAME",
			expectedEmptyCache: false,
			exit:               0,
		},
	}
	// A directory where we store the image and used by separate commands
	tempDir, imgStoreCleanup := e2e.MakeTempDir(t, "", "", "image store")
	defer imgStoreCleanup(t)
	imagePath := filepath.Join(tempDir, imgName)

	for _, tt := range tests {
		// Each test get its own clean cache directory
		cacheDir, cleanup := e2e.MakeCacheDir(t, "")
		defer cleanup(t)
		_, err := cache.New(cache.Config{ParentDir: cacheDir})
		if err != nil {
			t.Fatalf("Could not create image cache handle: %v", err)
		}

		imageURL := ""
		if tt.needImage {
			var srvCleanup func()
			imageURL, srvCleanup = prepTest(t, c.env, tt.name, cacheDir, imagePath)
			defer srvCleanup()
		}

		c.env.UnprivCacheDir = cacheDir
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("cache"),
			e2e.WithArgs(tt.options...),
			e2e.ExpectExit(tt.exit),
		)

		if tt.needImage && tt.expectedEmptyCache {
			ensureNotCached(t, tt.name, imageURL, cacheDir)
		}
	}
}

func (c cacheTests) testInteractiveCacheCmds(t *testing.T) {
	tt := []struct {
		name               string
		options            []string
		expect             string
		send               string
		exit               int
		expectedEmptyCache bool // Is the cache supposed to be empty after the command is executed
	}{
		{
			name:               "clean normal confirmed",
			options:            []string{"clean"},
			expect:             "Do you want to continue? [y/N]",
			send:               "y",
			expectedEmptyCache: true,
			exit:               0,
		},
		{
			name:               "clean normal not confirmed",
			options:            []string{"clean"},
			expect:             "Do you want to continue? [y/N]",
			send:               "n",
			expectedEmptyCache: false,
			exit:               0,
		},
		{
			name:               "clean normal force",
			options:            []string{"clean", "--force"},
			expectedEmptyCache: true,
			exit:               0,
		},
		{
			name:               "clean dry-run confirmed",
			options:            []string{"clean", "--dry-run"},
			expectedEmptyCache: false,
			exit:               0,
		},
		{
			name:               "clean type confirmed",
			options:            []string{"clean", "--type", "net"},
			expect:             "Do you want to continue? [y/N]",
			send:               "y",
			expectedEmptyCache: true,
			exit:               0,
		},
		{
			name:               "clean type not confirmed",
			options:            []string{"clean", "--type", "net"},
			expect:             "Do you want to continue? [y/N]",
			send:               "n",
			expectedEmptyCache: false,
			exit:               0,
		},
		{
			name:               "clean days beyond age",
			options:            []string{"clean", "--days", "30"},
			expect:             "Do you want to continue? [y/N]",
			send:               "y",
			expectedEmptyCache: false,
			exit:               0,
		},
		{
			name:               "clean days within age",
			options:            []string{"clean", "--days", "0"},
			expect:             "Do you want to continue? [y/N]",
			send:               "y",
			expectedEmptyCache: true,
			exit:               0,
		},
	}

	// A directory where we store the image and used by separate commands
	tempDir, imgStoreCleanup := e2e.MakeTempDir(t, "", "", "image store")
	defer imgStoreCleanup(t)
	imagePath := filepath.Join(tempDir, imgName)

	for _, tc := range tt {
		// Each test get its own clean cache directory
		cacheDir, cleanup := e2e.MakeCacheDir(t, "")
		defer cleanup(t)
		_, err := cache.New(cache.Config{ParentDir: cacheDir})
		if err != nil {
			t.Fatalf("Could not create image cache handle: %v", err)
		}

		c.env.UnprivCacheDir = cacheDir
		imageURL, srvCleanup := prepTest(t, c.env, tc.name, cacheDir, imagePath)
		defer srvCleanup()

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tc.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("cache"),
			e2e.WithArgs(tc.options...),
			e2e.ConsoleRun(
				e2e.ConsoleExpect(tc.expect),
				e2e.ConsoleSendLine(tc.send),
			),
			e2e.ExpectExit(tc.exit),
		)

		// Check the content of the cache
		if tc.expectedEmptyCache {
			ensureNotCached(t, tc.name, imageURL, cacheDir)
		} else {
			ensureCached(t, tc.name, imageURL, cacheDir)
		}
	}
}

func (c cacheTests) testMultipleArch(t *testing.T) {
	tempDir, tempcleanup := e2e.MakeTempDir(t, "", "", "sif build")
	defer tempcleanup(t)

	cacheDir, cleanup := e2e.MakeCacheDir(t, "")
	defer cleanup(t)
	_, err := cache.New(cache.Config{ParentDir: cacheDir})
	if err != nil {
		t.Fatalf("Could not create image cache handle: %v", err)
	}
	c.env.UnprivCacheDir = cacheDir

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("list cache"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("cache"),
		e2e.WithArgs([]string{"list", "all"}...),
		e2e.ExpectExit(0),
	)

	shamap := map[string]string{
		"arm64":      "ee74dfacff0fba9ec53b5f48dca94b1716b4e176425cb174fb1b58e973746ef8",
		"arm64v8":    "b118e86313b8da65a597ae0f773fb7cac49fac8e6666147a9443b172288c5306",
		"amd64":      "f38acf33dd020a91d4673e16c6ab14048b00a5fd83e334edd8b33d339103cf83",
		"arm32v6":    "9fe584fc821b9ceaf4e04cd21ed191dfa553517728da94e3f50632523b3ab374",
		"amd64uri":   "82e895f183a2f926f72ad8d28c3c36444ce37de4f9e1efeba35f4ed1643232d6",
		"arm64v8uri": "a101aa43bebd5f853837120fc8f28cae7ebe1c5f7f14a4d51a8c6d275f1456a9",
	}

	files := retrieveFileNames(t, cacheDir)
	if len(files) != 0 {
		t.Fatalf("Unexpected cache files: %v", files)
	}

	sifname := fmt.Sprintf("%s/build.sif", tempDir)

	// ko cases
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull image failure because of wrong --arch"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "amd", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(255, e2e.ExpectError(e2e.ContainMatch, "arch: amd is not valid")),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull image failure because --arch-variant is required"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "arm", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(255, e2e.ExpectError(e2e.ContainMatch, "arm needs variant specification")),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull image failure because of wrong --arch-variant"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "arm64", "--arch-variant", "v9", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(255, e2e.ExpectError(e2e.ContainMatch, "arch: arm64v9 is not valid")),
	)

	// ok cases
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull amd64 image"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "amd64", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(0),
	)
	ensureCachedWithSha(t, cacheDir, shamap["amd64"])

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull arm64 image"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "arm64", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(0),
	)
	ensureCachedWithSha(t, cacheDir, shamap["arm64"])

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull arm64 image"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "arm64", "--arch-variant", "v8", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(0),
	)
	ensureCachedWithSha(t, cacheDir, shamap["arm64v8"])

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull arm32v6 image"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "arm", "--arch-variant", "6", sifname, "docker://alpine:3.6"}...),
		e2e.ExpectExit(0),
	)
	ensureCachedWithSha(t, cacheDir, shamap["arm32v6"])

	files = retrieveFileNames(t, cacheDir)
	if len(files) != 4 {
		t.Fatalf("Unexpected cache files: %v", files)
	}

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull amd64 image by explicitly defining arch"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", sifname, "docker://amd64/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)
	ensureCachedWithSha(t, cacheDir, shamap["amd64uri"])

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull arm64 image by explicitly defining arch"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", sifname, "docker://arm64v8/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)
	ensureCachedWithSha(t, cacheDir, shamap["arm64v8uri"])

	files = retrieveFileNames(t, cacheDir)
	if len(files) != 6 {
		t.Fatalf("Unexpected cache files: %v", files)
	}

	// from now on, the cache should be hit when pulling
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull amd64 image using different arch and uri"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "386", sifname, "docker://amd64/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull amd64 image using different arch and uri with docker.io prefix"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "386", sifname, "docker://docker.io/amd64/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull arm64 image using different arch and uri"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "amd64", sifname, "docker://arm64v8/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pull arm64 image using different arch and uri with docker.io prefix"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs([]string{"--force", "--arch", "amd64", sifname, "docker://docker.io/arm64v8/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("build arm64 image using uri"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs([]string{"--force", sifname, "docker://arm64v8/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("build amd64 image using uri"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs([]string{"--force", sifname, "docker://amd64/alpine:3.6"}...),
		e2e.ExpectExit(0),
	)

	files = retrieveFileNames(t, cacheDir)
	if len(files) != 6 {
		t.Fatalf("Unexpected cache files: %v", files)
	}
}

// ensureNotCached checks the entry related to an image is not in the cache
func ensureNotCached(t *testing.T, testName string, imageURL string, cacheParentDir string) {
	shasum, err := netHash(imageURL)
	if err != nil {
		t.Fatalf("couldn't compute hash of image %s: %v", imageURL, err)
	}

	// Where the cached image should be
	cacheImagePath := path.Join(cacheParentDir, "cache", "net", shasum)

	// The image file shouldn't be present
	if e2e.PathExists(t, cacheImagePath) {
		t.Fatalf("%s failed: %s is still in the cache (%s)", testName, imgName, cacheImagePath)
	}
}

// ensureCached checks the entry related to an image is really in the cache
func ensureCached(t *testing.T, testName string, imageURL string, cacheParentDir string) {
	shasum, err := netHash(imageURL)
	if err != nil {
		t.Fatalf("couldn't compute hash of image %s: %v", imageURL, err)
	}

	// Where the cached image should be
	cacheImagePath := path.Join(cacheParentDir, "cache", "net", shasum)

	// The image file shouldn't be present
	if !e2e.PathExists(t, cacheImagePath) {
		t.Fatalf("%s failed: %s is not in the cache (%s)", testName, imgName, cacheImagePath)
	}
}

func ensureCachedWithSha(t *testing.T, cacheParentDir, shaval string) {
	cachePath := path.Join(cacheParentDir, "cache", "oci-tmp", shaval)
	if !e2e.PathExists(t, cachePath) {
		t.Fatalf("%s cache file does not exit", cachePath)
	}
}

func retrieveFileNames(t *testing.T, cacheParentDir string) []string {
	cachePath := path.Join(cacheParentDir, "cache", "oci-tmp")
	infos, err := os.ReadDir(cachePath)
	if err != nil {
		t.Fatalf("Failed to read the cache dir: %s", cachePath)
	}

	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name())
	}

	return names
}

// netHash computes the expected cache hash for the image at url
func netHash(url string) (hash string, err error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", fmt.Errorf("error constructing http request: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making http request: %w", err)
	}
	defer res.Body.Close()

	headerDate := res.Header.Get("Last-Modified")
	h := sha256.New()
	h.Write([]byte(url + headerDate))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := cacheTests{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"interactive commands":     np(c.testInteractiveCacheCmds),
		"non-interactive commands": np(c.testNoninteractiveCacheCmds),
		"issue5097":                np(c.issue5097),
		"issue5350":                np(c.issue5350),
		"test multiple archs":      np(c.testMultipleArch),
	}
}
