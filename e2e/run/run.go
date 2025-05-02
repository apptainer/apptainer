// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/cache"
)

type ctx struct {
	env e2e.TestEnv
}

// testRun555Cache tests the specific case where the cache directory is
// 0555 for access rights, and we try to run an Apptainer run command
// using that directory as cache. This reflects a problem that is important
// for the grid use case.
func (c ctx) testRun555Cache(t *testing.T) {
	e2e.EnsureORASImage(t, c.env)
	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)
	cacheDir := filepath.Join(tempDir, "image-cache")
	err := os.Mkdir(cacheDir, 0o555)
	if err != nil {
		t.Fatalf("failed to create a temporary image cache: %s", err)
	}
	// Directory is deleted when tempDir is deleted

	cmdArgs := []string{c.env.OrasTestImage, "true"}
	// We explicitly pass the environment to the command, not through c.env.ImgCacheDir
	// because c.env is shared between all the tests, something we do not want here.
	cacheDirEnv := fmt.Sprintf("%s=%s", cache.DirEnv, cacheDir)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), cacheDirEnv)),
		e2e.ExpectExit(0),
	)
}

func (c ctx) testRunPEMEncrypted(t *testing.T) {
	// If the version of cryptsetup is not compatible with Apptainer encryption,
	// the build commands are expected to fail
	err := e2e.CheckCryptsetupVersion()
	if err != nil {
		t.Skip("cryptsetup is not compatible, skipping test")
	}

	// It is too complicated right now to deal with a PEM file, the Sylabs infrastructure
	// does not let us attach one to a image in the library, so we generate one.
	pemPubFile, pemPrivFile := e2e.GeneratePemFiles(t, c.env.TestDir)

	// We create a temporary directory to store the image, making sure tests
	// will not pollute each other
	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	imgPath := filepath.Join(tempDir, "encrypted_cmdline_pem-path.sif")
	cmdArgs := []string{"--encrypt", "--pem-path", pemPubFile, imgPath, e2e.BusyboxSIF(t)}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(0),
	)

	// Using command line
	cmdArgs = []string{"--pem-path", pemPrivFile, imgPath}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pem file cmdline"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(0),
	)

	// Using environment variable
	cmdArgs = []string{imgPath}
	pemEnvVar := fmt.Sprintf("%s=%s", "APPTAINER_ENCRYPTION_PEM_PATH", pemPrivFile)
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("pem file cmdline"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), pemEnvVar)),
		e2e.ExpectExit(0),
	)
}

func (c ctx) testRunPassphraseEncrypted(t *testing.T) {
	// If the version of cryptsetup is not compatible with Apptainer encryption,
	// the build commands are expected to fail
	err := e2e.CheckCryptsetupVersion()
	if err != nil {
		t.Skip("cryptsetup is not compatible, skipping test")
	}

	passphraseEnvVar := fmt.Sprintf("%s=%s", "APPTAINER_ENCRYPTION_PASSPHRASE", e2e.Passphrase)

	// We create a temporary directory to store the image, making sure tests
	// will not pollute each other
	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	imgPath := filepath.Join(tempDir, "encrypted_cmdline_passphrase.sif")
	cmdArgs := []string{"--encrypt", imgPath, e2e.BusyboxSIF(t)}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(0),
	)

	passphraseInput := []e2e.ApptainerConsoleOp{
		e2e.ConsoleSendLine(e2e.Passphrase),
	}

	// Interactive command
	cmdArgs = []string{"--passphrase", imgPath, "/bin/true"}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("interactive passphrase"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(cmdArgs...),
		e2e.ConsoleRun(passphraseInput...),
		e2e.ExpectExit(0),
	)

	// Using the environment variable to specify the passphrase
	cmdArgs = []string{imgPath}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("env var passphrase"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(0),
	)

	// Ensure decryption works with an IPC namespace
	cmdArgs = []string{"--ipc", imgPath}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("env var passphrase with ipc namespace"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(0),
	)

	// Ensure decryption works with containall (IPC and PID namespaces)
	cmdArgs = []string{"--containall", imgPath}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("env var passphrase with containall"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(0),
	)

	// Specifying the passphrase on the command line should always fail
	cmdArgs = []string{"--passphrase", e2e.Passphrase, imgPath}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("passphrase on cmdline"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(255),
	)
}

func (c ctx) testFuseOverlayfs(t *testing.T) {
	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	overlayPath := fmt.Sprintf("%s/overlay.img", tempDir)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("overlay"),
		e2e.WithArgs("create", "--size", "64", overlayPath),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserNamespaceProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--writable-tmpfs", "testdata/busybox_amd64.sif", "touch", "file"),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserNamespaceProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--overlay", overlayPath, "testdata/busybox_amd64.sif", "touch", "file"),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserNamespaceProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--overlay", tempDir, "testdata/busybox_amd64.sif", "touch", "file"),
		e2e.ExpectExit(0),
	)
}

func (c ctx) testFuseSquashMount(t *testing.T) {
	dataDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	file, err := os.CreateTemp(dataDir, "")
	if err != nil {
		t.Fatalf("failed to create temp file under temp data dir: %s", dataDir)
	}

	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	filename := file.Name()
	file.Close()

	squashfile := fmt.Sprintf("%s/input.squashfs", tempDir)
	_, err = exec.Command("mksquashfs", dataDir, squashfile).Output()
	if err != nil {
		t.Fatalf("%v", err.Error())
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--mount", fmt.Sprintf("type=bind,src=%s,dst=/input-data,image-src=/", squashfile), "testdata/busybox_amd64.sif", "ls", "/input-data"),
		e2e.ExpectExit(0, e2e.ExpectOutput(e2e.ContainMatch, filepath.Base(filename))),
	)
}

func (c ctx) testFuseExt3Mount(t *testing.T) {
	dataDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	file, err := os.CreateTemp(dataDir, "")
	if err != nil {
		t.Fatalf("failed to create temp file under temp data dir: %s", dataDir)
	}

	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	filename := file.Name()
	file.Close()

	ext3file := fmt.Sprintf("%s/input.img", tempDir)
	_, err = exec.Command("mkfs.ext3", "-d", dataDir, ext3file, "64M").Output()
	if err != nil {
		t.Fatalf("%v", err.Error())
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--mount", fmt.Sprintf("type=bind,src=%s,dst=/input-data,image-src=/", ext3file), "testdata/busybox_amd64.sif", "ls", "/input-data"),
		e2e.ExpectExit(0, e2e.ExpectOutput(e2e.ContainMatch, filepath.Base(filename))),
	)
}

func (c ctx) testAddPackageWithFakerootAndTmpfs(t *testing.T) {
	e2e.EnsureDebianImage(t, c.env)

	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer e2e.Privileged(cleanup)(t)

	sandbox, err := os.MkdirTemp(tempDir, "sandbox")
	if err != nil {
		t.Fatalf("could not create sandbox folder inside tempdir: %s", tempDir)
	}

	sif := c.env.DebianImagePath

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--sandbox", "--force", sandbox, sif),
		e2e.ExpectExit(0),
	)

	// we need to increase sessiondir max size to 1GB
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("config"),
		e2e.WithArgs("global", "-s", "sessiondir max size", "1024"),
		e2e.ExpectExit(0),
	)

	// restore sessiondir max size to 64MB on exit
	defer c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("config"),
		e2e.WithArgs("global", "-s", "sessiondir max size", "64"),
		e2e.ExpectExit(0),
	)

	// running under the mode 1, 1a (--with-suid) (https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--writable-tmpfs", sif, "sh", "-c", "apt-get update && apt-get install -y openssh-client"),
		e2e.ExpectExit(0),
	)

	// running under the mode 1, 1b (--without-suid) (https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--userns", "--writable-tmpfs", sif, "sh", "-c", "apt-get update && apt-get install -y openssh-client"),
		e2e.ExpectExit(0),
	)

	// running under the mode 2(https://apptainer.org/docs/user/main/fakeroot.html)
	// which can't handle installing openssh
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--userns", "--writable-tmpfs", "--ignore-subuid", "--ignore-fakeroot-command", sif, "sh", "-c", "apt-get update && apt-get install -y openssh-client"),
		e2e.ExpectExit(100),
	)

	// mode 2 can't handle installing packages on Debian but it can override permissions
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--userns", "--writable-tmpfs", "--ignore-subuid", "--ignore-fakeroot-command", sif, "sh", "-c", "mkdir -m 0 /etc/denied && ls /etc/denied"),
		e2e.ExpectExit(0),
	)

	// running under the mode 3(https://apptainer.org/docs/user/main/fakeroot.html)
	overlaydir, err := os.MkdirTemp(tempDir, "overlaymode3")
	if err != nil {
		t.Fatalf("could not create overlaymode3 folder inside tempdir: %s", tempDir)
	}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		// This works with --writable-tmpfs interactively, but somehow
		// gives postinstall errors about a missing 'diff' command when
		// using it on ubuntu 22.04, so use overlay instead.
		// See https://github.com/apptainer/apptainer/issues/1124
		e2e.WithArgs("--userns", "--overlay", overlaydir, "--ignore-subuid", sif, "sh", "-c", "apt-get update && apt-get install -y openssh-client"),
		// e2e.WithArgs("--userns", "--writable-tmpfs", "--ignore-subuid", sif, "sh", "-c", "apt-get update && apt-get install -y openssh-client"),
		e2e.ExpectExit(0),
	)

	// running under the mode 4(https://apptainer.org/docs/user/main/fakeroot.html)
	// which can install a simple package, when using --writable
	// but not when using --writable-tmpfs (because the fuse-overlayfs
	// in between refuses to accept faking the operations)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--writable", "--ignore-userns", "--ignore-subuid", sandbox, "sh", "-c", "apt-get update && apt-get install -y cpio"),
		e2e.ExpectExit(0),
	)

	// mode 4 however cannot install the more complex package openssh-client
	// NOTE: this must be the last thing attempted to be installed into the
	// sandbox because subsequent installs will try to complete this
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--no-home", "--writable", "--ignore-userns", "--ignore-subuid", sandbox, "sh", "-c", "apt-get update && apt-get install -y openssh-client"),
		e2e.ExpectExit(100), // fails because only fakeroot is used. error with addgroup.
	)
}

func (c ctx) testExecGocryptfsEncryptedSIF(t *testing.T) {
	pemPubFile, pemPrivFile := e2e.GeneratePemFiles(t, c.env.TestDir)
	// We create a temporary directory to store the image, making sure tests
	// will not pollute each other
	tempDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
	defer cleanup(t)

	imgPath := filepath.Join(tempDir, "encrypted_pem.sif")
	cmdArgs := []string{"--pem-path", pemPubFile, imgPath, e2e.BusyboxSIF(t)}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(0),
	)

	// Using command line
	cmdArgs = []string{"--userns", "--pem-path", pemPrivFile, imgPath, "sh", "-c", "echo 'hi'"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "hi"),
		),
	)

	// Using environment variables
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv([]string{fmt.Sprintf("APPTAINER_ENCRYPTION_PEM_PATH=%s", pemPrivFile)}),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "hi"),
		),
	)

	imgPassPath := filepath.Join(tempDir, "encrypted_pass.sif")
	cmdArgs = []string{"--userns", "--passphrase", imgPassPath, e2e.BusyboxSIF(t)}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithStdin(strings.NewReader("1234\n")),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(0),
	)

	// Using command line
	cmdArgs = []string{"--userns", "--passphrase", imgPassPath, "sh", "-c", "echo 'hi'"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithStdin(strings.NewReader("1234\n")),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "hi"),
		),
	)

	// Using environment variables
	cmdArgs = []string{"--userns", imgPassPath, "sh", "-c", "echo 'hi'"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv([]string{"APPTAINER_ENCRYPTION_PASSPHRASE=1234"}),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "hi"),
		),
	)

	// Using both command line and environment variables
	cmdArgs = []string{"--userns", "--passphrase", imgPassPath, "sh", "-c", "echo 'hi'"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv([]string{"APPTAINER_ENCRYPTION_PASSPHRASE=1234"}),
		e2e.WithStdin(strings.NewReader("1234\n")),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "hi"),
		),
	)
}

func (c ctx) testMultiArchRun(t *testing.T) {
	imgPath := e2e.BusyboxSIFArch(t, "amd64")
	cmdArgs := []string{"--cleanenv", imgPath, "uname", "-m"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "x86_64"),
		),
	)

	imgPath = e2e.BusyboxSIFArch(t, "arm64")
	cmdArgs = []string{"--cleanenv", imgPath, "uname", "-m"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "aarch64"),
		),
	)

	imgPath = e2e.BusyboxSIFArch(t, "ppc64le")
	cmdArgs = []string{"--cleanenv", imgPath, "uname", "-m"}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, "ppc64le"),
		),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"0555 cache":                          c.testRun555Cache,
		"inaccessible home":                   c.issue409,
		"passphrase encrypted":                c.testRunPassphraseEncrypted,
		"PEM encrypted":                       c.testRunPEMEncrypted,
		"fuse overlayfs":                      c.testFuseOverlayfs,
		"fuse squash mount":                   c.testFuseSquashMount,
		"fuse ext3 mount":                     c.testFuseExt3Mount,
		"add package with fakeroot and tmpfs": c.testAddPackageWithFakerootAndTmpfs,
		"gocryptfs sif execution":             c.testExecGocryptfsEncryptedSIF,
		"test running on multiple archs":      c.testMultiArchRun,
	}
}
