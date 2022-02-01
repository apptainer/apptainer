// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019,2020 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"testing"

	// Tests imports
	"github.com/apptainer/apptainer/e2e/actions"
	e2ebuildcfg "github.com/apptainer/apptainer/e2e/buildcfg"
	"github.com/apptainer/apptainer/e2e/cache"
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
	name, err := ioutil.TempDir("", "stest.")
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

	// e2e tests need to run in a somehow agnostic environment, so we
	// don't use environment of user executing tests in order to not
	// wrongly interfering with cache stuff, sylabs library tokens,
	// PGP keys
	e2e.SetupHomeDirectories(t)

	// generate apptainer.conf with default values
	e2e.SetupDefaultConfig(t, filepath.Join(testenv.TestDir, "apptainer.conf"))

	// create an empty plugin directory
	e2e.SetupPluginDir(t, testenv.TestDir)

	// duplicate system remote.yaml and create a temporary one on top of original
	e2e.SetupSystemRemoteFile(t, testenv.TestDir)

	// create an empty ECL configuration and empty global keyring
	e2e.SetupSystemECLAndGlobalKeyRing(t, testenv.TestDir)

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
			log.Fatalf("%s is not installed on this system: %v", cf, err)
		} else if !fi.Mode().IsRegular() {
			log.Fatalf("%s is not a regular file", cf)
		} else if fi.Sys().(*syscall.Stat_t).Uid != 0 {
			log.Fatalf("%s must be owned by root", cf)
		}
	}

	// Build a base image for tests
	imagePath := path.Join(name, "test.sif")
	t.Log("Path to test image:", imagePath)
	testenv.ImagePath = imagePath
	defer os.Remove(imagePath)

	testenv.SingularityImagePath = path.Join(name, "test-singularity.sif")
	defer os.Remove(testenv.SingularityImagePath)

	// WARNING(Sylabs-team): Please DO NOT add a call to e2e.EnsureImage here.
	// If you need the test image, add the call at the top of your
	// own test.

	testenv.TestRegistry = "localhost:5000"
	testenv.OrasTestImage = fmt.Sprintf("oras://%s/oras_test_sif:latest", testenv.TestRegistry)

	// Because tests are parallelized, and PrepRegistry temporarily masks
	// the Apptainer instance directory we *must* now call it before we
	// start running tests which could use instance and oci functionality.
	// See: https://github.com/apptainer/singularity/issues/5744
	t.Run("PrepRegistry", func(t *testing.T) {
		e2e.PrepRegistry(t, testenv)
	})
	// e2e.KillRegistry is called here to ensure that the registry
	// is stopped after tests run.
	defer e2e.KillRegistry(t, testenv)

	suite := testhelper.NewSuite(t, testenv)

	// RunE2ETests by functionality.
	//
	// Please keep this list sorted.
	suite.AddGroup("ACTIONS", actions.E2ETests)
	suite.AddGroup("BUILDCFG", e2ebuildcfg.E2ETests)
	suite.AddGroup("BUILD", imgbuild.E2ETests)
	suite.AddGroup("CACHE", cache.E2ETests)
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
