// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2022 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	// Tests imports
	"github.com/apptainer/apptainer/e2e/actions"
	e2ebuildcfg "github.com/apptainer/apptainer/e2e/buildcfg"
	"github.com/apptainer/apptainer/e2e/cache"
	"github.com/apptainer/apptainer/e2e/cgroups"
	"github.com/apptainer/apptainer/e2e/cmdenvvars"
	"github.com/apptainer/apptainer/e2e/config"
	"github.com/apptainer/apptainer/e2e/delete"
	"github.com/apptainer/apptainer/e2e/docker"
	"github.com/apptainer/apptainer/e2e/ecl"
	apptainerenv "github.com/apptainer/apptainer/e2e/env"
	"github.com/apptainer/apptainer/e2e/gpu"
	"github.com/apptainer/apptainer/e2e/help"
	"github.com/apptainer/apptainer/e2e/imgbuild"
	"github.com/apptainer/apptainer/e2e/inspect"
	"github.com/apptainer/apptainer/e2e/instance"
	"github.com/apptainer/apptainer/e2e/key"
	"github.com/apptainer/apptainer/e2e/legacy"
	"github.com/apptainer/apptainer/e2e/oci"
	"github.com/apptainer/apptainer/e2e/overlay"
	"github.com/apptainer/apptainer/e2e/plugin"
	"github.com/apptainer/apptainer/e2e/pull"
	"github.com/apptainer/apptainer/e2e/push"
	"github.com/apptainer/apptainer/e2e/remote"
	"github.com/apptainer/apptainer/e2e/run"
	"github.com/apptainer/apptainer/e2e/runhelp"
	"github.com/apptainer/apptainer/e2e/security"
	"github.com/apptainer/apptainer/e2e/sign"
	"github.com/apptainer/apptainer/e2e/verify"
	"github.com/apptainer/apptainer/e2e/version"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

var runDisabled = flag.Bool("run_disabled", false, "run tests that have been temporarily disabled")

// Run is the main func for the test framework, initializes the required vars
// and sets the environment for the RunE2ETests framework
func Run(t *testing.T) {
	flag.Parse()

	var testenv e2e.TestEnv

	if *runDisabled {
		testenv.RunDisabled = true
	}
	// init buildcfg values
	useragent.InitValue(buildcfg.PACKAGE_NAME, buildcfg.PACKAGE_VERSION)

	// Ensure binary is in $PATH
	cmdPath := filepath.Join(buildcfg.BINDIR, "apptainer")
	if _, err := exec.LookPath(cmdPath); err != nil {
		log.Fatalf("apptainer is not installed on this system: %v", err)
	}

	testenv.CmdPath = cmdPath

	sysconfdir := func(fn string) string {
		return filepath.Join(buildcfg.SYSCONFDIR, "apptainer", fn)
	}

	// Make temp dir for tests
	name, err := os.MkdirTemp("", "stest.")
	if err != nil {
		log.Fatalf("failed to create temporary directory: %v", err)
	}
	defer e2e.Privileged(func(t *testing.T) {
		if t.Failed() {
			t.Logf("Test failed, not removing %s", name)
			return
		}

		os.RemoveAll(name)
	})(t)

	if err := os.Chmod(name, 0o755); err != nil {
		log.Fatalf("failed to chmod temporary directory: %v", err)
	}
	testenv.TestDir = name
	testenv.TestRegistry = e2e.StartRegistry(t, testenv)
	testenv.InsecureRegistry = strings.Replace(testenv.TestRegistry, "localhost", "127.0.0.1.nip.io", 1)

	// Make shared cache dirs for privileged and unprivileged E2E tests.
	// Individual tests that depend on specific ordered cache behavior, or
	// directly test the cache, should override the TestEnv values within the
	// specific test.
	privCacheDir, cleanPrivCache := e2e.MakeCacheDir(t, testenv.TestDir)
	testenv.PrivCacheDir = privCacheDir
	defer e2e.Privileged(func(t *testing.T) {
		cleanPrivCache(t)
	})

	unprivCacheDir, cleanUnprivCache := e2e.MakeCacheDir(t, testenv.TestDir)
	testenv.UnprivCacheDir = unprivCacheDir
	defer cleanUnprivCache(t)

	// e2e tests need to run in a somehow agnostic environment, so we
	// don't use environment of user executing tests in order to not
	// wrongly interfering with cache stuff, sylabs library tokens,
	// PGP keys
	e2e.SetupHomeDirectories(t, testenv.TestRegistry)

	// generate apptainer.conf with default values
	e2e.SetupDefaultConfig(t, filepath.Join(testenv.TestDir, "apptainer.conf"))

	// create an empty plugin directory
	e2e.SetupPluginDir(t, testenv.TestDir)

	// duplicate system remote.yaml and create a temporary one on top of original
	e2e.SetupSystemRemoteFile(t, testenv.TestDir)

	// create an empty ECL configuration and empty global keyring
	e2e.SetupSystemECLAndGlobalKeyRing(t, testenv.TestDir)

	// Creates '$HOME/.apptainer/docker-config.json' with credentials
	e2e.SetupDockerHubCredentials(t)

	// Ensure config files are installed
	configFiles := []string{
		sysconfdir("apptainer.conf"),
		sysconfdir("ecl.toml"),
		sysconfdir("capability.json"),
		sysconfdir("nvliblist.conf"),
	}

	for _, cf := range configFiles {
		if fi, err := os.Stat(cf); err != nil {
			t.Fatalf("%s is not installed on this system: %v", cf, err)
		} else if !fi.Mode().IsRegular() {
			t.Fatalf("%s is not a regular file", cf)
		} else if fi.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatalf("%s must be owned by root", cf)
		}
	}

	testenv.SingularityImagePath = path.Join(name, "test-singularity.sif")
	defer os.Remove(testenv.SingularityImagePath)

	testenv.DebianImagePath = path.Join(name, "test-debian.sif")
	defer os.Remove(testenv.DebianImagePath)

	testenv.OrasTestImage = fmt.Sprintf("oras://%s/oras_test_sif:latest", testenv.TestRegistry)

	// Provision local registry
	testenv.TestRegistryImage = fmt.Sprintf("docker://%s/my-busybox:latest", testenv.TestRegistry)

	// Copy small test image (busybox:latest) into local registry from DockerHub
	insecureSource := false
	insecureValue := os.Getenv("E2E_DOCKER_MIRROR_INSECURE")
	if insecureValue != "" {
		insecureSource, err = strconv.ParseBool(insecureValue)
		if err != nil {
			t.Fatalf("could not convert E2E_DOCKER_MIRROR_INSECURE=%s: %s", insecureValue, err)
		}
	}
	e2e.CopyImage(t, "docker://busybox:latest", testenv.TestRegistryImage, insecureSource, true)

	// SIF base test path, built on demand by e2e.EnsureImage
	imagePath := path.Join(name, "test.sif")
	t.Log("Path to test image:", imagePath)
	testenv.ImagePath = imagePath

	// Local registry ORAS SIF image, built on demand by e2e.EnsureORASImage
	testenv.OrasTestImage = fmt.Sprintf("oras://%s/oras_test_sif:latest", testenv.TestRegistry)

	t.Cleanup(func() {
		os.Remove(imagePath)
	})

	suite := testhelper.NewSuite(t, testenv)

	suite.AddGroup("ACTIONS", actions.E2ETests)
	suite.AddGroup("BUILDCFG", e2ebuildcfg.E2ETests)
	suite.AddGroup("BUILD", imgbuild.E2ETests)
	suite.AddGroup("CACHE", cache.E2ETests)
	suite.AddGroup("CGROUPS", cgroups.E2ETests)
	suite.AddGroup("CMDENVVARS", cmdenvvars.E2ETests)
	suite.AddGroup("CONFIG", config.E2ETests)
	suite.AddGroup("DELETE", delete.E2ETests)
	suite.AddGroup("DOCKER", docker.E2ETests)
	suite.AddGroup("ECL", ecl.E2ETests)
	suite.AddGroup("ENV", apptainerenv.E2ETests)
	suite.AddGroup("GPU", gpu.E2ETests)
	suite.AddGroup("HELP", help.E2ETests)
	suite.AddGroup("INSPECT", inspect.E2ETests)
	suite.AddGroup("INSTANCE", instance.E2ETests)
	suite.AddGroup("KEY", key.E2ETests)
	suite.AddGroup("LEGACY", legacy.E2ETests)
	suite.AddGroup("OCI", oci.E2ETests)
	suite.AddGroup("OVERLAY", overlay.E2ETests)
	suite.AddGroup("PLUGIN", plugin.E2ETests)
	suite.AddGroup("PULL", pull.E2ETests)
	suite.AddGroup("PUSH", push.E2ETests)
	suite.AddGroup("REMOTE", remote.E2ETests)
	suite.AddGroup("RUN", run.E2ETests)
	suite.AddGroup("RUNHELP", runhelp.E2ETests)
	suite.AddGroup("SECURITY", security.E2ETests)
	suite.AddGroup("SIGN", sign.E2ETests)
	suite.AddGroup("VERIFY", verify.E2ETests)
	suite.AddGroup("VERSION", version.E2ETests)
	suite.Run()
}
