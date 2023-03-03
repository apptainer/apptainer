// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// The DOCKER E2E group tests functionality of actions, pulls / builds of
// Docker/OCI source images. These tests are separated from direct SIF build /
// pull / actions because they examine OCI specific image behavior. They are run
// ordered, rather than in parallel to avoid any concurrency issues with
// containers/image. Also, we can then maximally benefit from caching to avoid
// Docker Hub rate limiting.

package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	dockerclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type ctx struct {
	env e2e.TestEnv
}

func (c ctx) testDockerPulls(t *testing.T) {
	const tmpContainerFile = "test_container.sif"

	tmpPath, err := fs.MakeTmpDir(c.env.TestDir, "docker-", 0o755)
	err = errors.Wrapf(err, "creating temporary directory in %q for docker pull test", c.env.TestDir)
	if err != nil {
		t.Fatalf("failed to create temporary directory: %+v", err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			os.RemoveAll(tmpPath)
		}
	})

	tmpImage := filepath.Join(tmpPath, tmpContainerFile)

	tests := []struct {
		name    string
		options []string
		image   string
		uri     string
		exit    int
	}{
		{
			name:  "BusyboxLatestPull",
			image: tmpImage,
			uri:   "docker://busybox:latest",
			exit:  0,
		},
		{
			name:  "BusyboxLatestPullFail",
			image: tmpImage,
			uri:   "docker://busybox:latest",
			exit:  255,
		},
		{
			name:    "BusyboxLatestPullForce",
			options: []string{"--force"},
			image:   tmpImage,
			uri:     "docker://busybox:latest",
			exit:    0,
		},
		{
			name:    "Busybox1.28Pull",
			options: []string{"--force", "--dir", tmpPath},
			image:   tmpContainerFile,
			uri:     "docker://busybox:1.28",
			exit:    0,
		},
		{
			name:  "Busybox1.28PullFail",
			image: tmpImage,
			uri:   "docker://busybox:1.28",
			exit:  255,
		},
		{
			name:  "Busybox1.28PullDirFail",
			image: "/foo/sif.sif",
			uri:   "docker://busybox:1.28",
			exit:  255,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("pull"),
			e2e.WithArgs(append(tt.options, tt.image, tt.uri)...),
			e2e.PostRun(func(t *testing.T) {
				if !t.Failed() && tt.exit == 0 {
					path := tt.image
					// handle the --dir case
					if path == tmpContainerFile {
						path = filepath.Join(tmpPath, tmpContainerFile)
					}
					c.env.ImageVerify(t, path, e2e.UserProfile)
				}
			}),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// Testing DOCKER_ host support (only if docker available)
func (c ctx) testDockerHost(t *testing.T) {
	require.Command(t, "docker")

	// Temporary homedir for docker commands, so invoking docker doesn't create
	// a ~/.docker that may interfere elsewhere.
	tmpHome, cleanupHome := e2e.MakeTempDir(t, c.env.TestDir, "docker-", "")
	t.Cleanup(func() { e2e.Privileged(cleanupHome)(t) })

	// Create a Dockerfile for a small image we can build locally
	tmpPath, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "docker-", "")
	t.Cleanup(func() { cleanup(t) })

	dockerfile := filepath.Join(tmpPath, "Dockerfile")
	dockerfileContent := []byte("FROM alpine:latest\n")
	err := os.WriteFile(dockerfile, dockerfileContent, 0o644)
	if err != nil {
		t.Fatalf("failed to create temporary Dockerfile: %+v", err)
	}

	dockerRef := "dinosaur/test-image:latest"
	dockerURI := "docker-daemon:" + dockerRef

	// Invoke docker build to build image in the docker daemon.
	// Use os/exec because easier to generate a command with a working directory
	e2e.Privileged(func(t *testing.T) {
		cmd := exec.Command("docker", "build", "-t", dockerRef, tmpPath)
		cmd.Dir = tmpPath
		cmd.Env = append(cmd.Env, "HOME="+tmpHome)
		out, err := cmd.CombinedOutput()
		t.Log(cmd.Args)
		if err != nil {
			t.Fatalf("Unexpected error while running command.\n%s: %s", err, string(out))
		}
	})(t)

	tests := []struct {
		name       string
		envarName  string
		envarValue string
		exit       int
	}{
		// Unset docker host should use default and succeed
		{
			name:       "apptainerDockerHostEmpty",
			envarName:  "APPTAINER_DOCKER_HOST",
			envarValue: "",
			exit:       0,
		},
		{
			name:       "dockerHostEmpty",
			envarName:  "DOCKER_HOST",
			envarValue: "",
			exit:       0,
		},

		// bad Docker host should fail
		{
			name:       "apptainerDockerHostInvalid",
			envarName:  "APPTAINER_DOCKER_HOST",
			envarValue: "tcp://192.168.59.103:oops",
			exit:       255,
		},
		{
			name:       "dockerHostInvalid",
			envarName:  "DOCKER_HOST",
			envarValue: "tcp://192.168.59.103:oops",
			exit:       255,
		},

		// Set to default should succeed
		// The default host varies based on OS, so we use dockerclient default
		{
			name:       "apptainerDockerHostValid",
			envarName:  "APPTAINER_DOCKER_HOST",
			envarValue: dockerclient.DefaultDockerHost,
			exit:       0,
		},
		{
			name:       "dockerHostValid",
			envarName:  "DOCKER_HOST",
			envarValue: dockerclient.DefaultDockerHost,
			exit:       0,
		},
	}

	t.Run("exec", func(t *testing.T) {
		for _, tt := range tests {
			cmdOps := []e2e.ApptainerCmdOp{
				e2e.WithProfile(e2e.RootProfile),
				e2e.AsSubtest(tt.name),
				e2e.WithCommand("exec"),
				e2e.WithArgs("--disable-cache", dockerURI, "/bin/true"),
				e2e.WithEnv(append(os.Environ(), tt.envarName+"="+tt.envarValue)),
				e2e.ExpectExit(tt.exit),
			}
			c.env.RunApptainer(t, cmdOps...)
		}
	})

	t.Run("pull", func(t *testing.T) {
		for _, tt := range tests {
			cmdOps := []e2e.ApptainerCmdOp{
				e2e.WithProfile(e2e.RootProfile),
				e2e.AsSubtest(tt.name),
				e2e.WithCommand("pull"),
				e2e.WithArgs("--force", "--disable-cache", dockerURI),
				e2e.WithEnv(append(os.Environ(), tt.envarName+"="+tt.envarValue)),
				e2e.WithDir(tmpPath),
				e2e.ExpectExit(tt.exit),
			}
			c.env.RunApptainer(t, cmdOps...)
		}
	})

	t.Run("build", func(t *testing.T) {
		for _, tt := range tests {
			cmdOps := []e2e.ApptainerCmdOp{
				e2e.WithProfile(e2e.RootProfile),
				e2e.AsSubtest(tt.name),
				e2e.WithCommand("build"),
				e2e.WithArgs("--force", "--disable-cache", "test.sif", dockerURI),
				e2e.WithEnv(append(os.Environ(), tt.envarName+"="+tt.envarValue)),
				e2e.WithDir(tmpPath),
				e2e.ExpectExit(tt.exit),
			}
			c.env.RunApptainer(t, cmdOps...)
		}
	})

	// Clean up docker image
	e2e.Privileged(func(t *testing.T) {
		cmd := exec.Command("docker", "rmi", dockerRef)
		cmd.Env = append(cmd.Env, "HOME="+tmpHome)
		_, err = cmd.Output()
		if err != nil {
			t.Fatalf("Unexpected error while cleaning up docker image.\n%s", err)
		}
	})(t)
}

// AUFS sanity tests
func (c ctx) testDockerAUFS(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "aufs-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs([]string{imagePath, "docker://ghcr.io/apptainer/aufs-sanity"}...),
		e2e.ExpectExit(0),
	)

	if t.Failed() {
		return
	}

	fileTests := []struct {
		name string
		argv []string
		exit int
	}{
		{
			name: "File 2",
			argv: []string{imagePath, "ls", "/test/whiteout-dir/file2", "/test/whiteout-file/file2", "/test/normal-dir/file2"},
			exit: 0,
		},
		{
			name: "File1",
			argv: []string{imagePath, "ls", "/test/whiteout-dir/file1", "/test/whiteout-file/file1"},
			exit: 1,
		},
	}

	for _, tt := range fileTests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// Check force permissions for user builds #977
func (c ctx) testDockerPermissions(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "perm-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs([]string{imagePath, "docker://ghcr.io/apptainer/userperms"}...),
		e2e.ExpectExit(0),
	)

	if t.Failed() {
		return
	}

	fileTests := []struct {
		name string
		argv []string
		exit int
	}{
		{
			name: "TestDir",
			argv: []string{imagePath, "ls", "/testdir/"},
			exit: 0,
		},
		{
			name: "TestDirFile",
			argv: []string{imagePath, "ls", "/testdir/testfile"},
			exit: 1,
		},
	}
	for _, tt := range fileTests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// Check whiteout of symbolic links #1592 #1576
func (c ctx) testDockerWhiteoutSymlink(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "whiteout-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs([]string{imagePath, "docker://ghcr.io/apptainer/linkwh"}...),
		e2e.PostRun(func(t *testing.T) {
			if t.Failed() {
				return
			}
			c.env.ImageVerify(t, imagePath, e2e.UserProfile)
		}),
		e2e.ExpectExit(0),
	)
}

func (c ctx) testDockerDefFile(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "def-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	getKernelMajor := func(t *testing.T) (major int) {
		var buf unix.Utsname
		if err := unix.Uname(&buf); err != nil {
			err = errors.Wrap(err, "getting current kernel information")
			t.Fatalf("uname failed: %+v", err)
		}
		n, err := fmt.Sscanf(string(buf.Release[:]), "%d.", &major)
		err = errors.Wrap(err, "getting current kernel release")
		if err != nil {
			t.Fatalf("Sscanf failed, n=%d: %+v", n, err)
		}
		if n != 1 {
			t.Fatalf("Unexpected result while getting major release number: n=%d", n)
		}
		return
	}

	tests := []struct {
		name                string
		kernelMajorRequired int
		archRequired        string
		from                string
	}{
		{
			name:                "Alpine",
			kernelMajorRequired: 0,
			from:                "alpine:latest",
		},
		{
			name:                "RockyLinux_9",
			kernelMajorRequired: 3,
			from:                "rockylinux:9",
		},
		{
			name:                "Ubuntu_2204",
			kernelMajorRequired: 3,
			from:                "ubuntu:22.04",
		},
	}

	for _, tt := range tests {
		defFile := e2e.PrepareDefFile(e2e.DefFileDetails{
			Bootstrap: "docker",
			From:      tt.from,
		})

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs([]string{imagePath, defFile}...),
			e2e.PreRun(func(t *testing.T) {
				require.Arch(t, tt.archRequired)
				if getKernelMajor(t) < tt.kernelMajorRequired {
					t.Skipf("kernel >=%v.x required", tt.kernelMajorRequired)
				}
			}),
			e2e.PostRun(func(t *testing.T) {
				if t.Failed() {
					return
				}

				c.env.ImageVerify(t, imagePath, e2e.RootProfile)

				if !t.Failed() {
					os.Remove(imagePath)
					os.Remove(defFile)
				}
			}),
			e2e.ExpectExit(0),
		)
	}
}

func (c ctx) testDockerRegistry(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "registry-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	tests := []struct {
		name string
		exit int
		dfd  e2e.DefFileDetails
	}{
		{
			name: "BusyBox",
			exit: 0,
			dfd: e2e.DefFileDetails{
				Bootstrap: "docker",
				From:      fmt.Sprintf("%s/my-busybox", c.env.TestRegistry),
			},
		},
		{
			name: "BusyBoxRegistry",
			exit: 0,
			dfd: e2e.DefFileDetails{
				Bootstrap: "docker",
				From:      "my-busybox",
				Registry:  c.env.TestRegistry,
			},
		},
		{
			name: "BusyBoxNamespace",
			exit: 255,
			dfd: e2e.DefFileDetails{
				Bootstrap: "docker",
				From:      "my-busybox",
				Registry:  c.env.TestRegistry,
				Namespace: "not-a-namespace",
			},
		},
	}

	for _, tt := range tests {
		defFile := e2e.PrepareDefFile(tt.dfd)

		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs("--disable-cache", "--no-https", imagePath, defFile),
			e2e.PostRun(func(t *testing.T) {
				if t.Failed() || tt.exit != 0 {
					return
				}

				c.env.ImageVerify(t, imagePath, e2e.RootProfile)

				if !t.Failed() {
					os.Remove(imagePath)
					os.Remove(defFile)
				}
			}),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// https://github.com/sylabs/singularity/issues/233
func (c ctx) testDockerCMDQuotes(t *testing.T) {
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs("docker://ghcr.io/apptainer/issue233"),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "Test run"),
		),
	)
}

func (c ctx) testDockerLabels(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "labels-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	// Test container & set labels
	// See: https://github.com/sylabs/singularity-test-containers/pull/1
	imgSrc := "docker://ghcr.io/apptainer/labels"
	label1 := "LABEL1: 1"
	label2 := "LABEL2: TWO"

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("build"),
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(imagePath, imgSrc),
		e2e.ExpectExit(0),
	)

	verifyOutput := func(t *testing.T, r *e2e.ApptainerCmdResult) {
		output := string(r.Stdout)
		for _, l := range []string{label1, label2} {
			if !strings.Contains(output, l) {
				t.Errorf("Did not find expected label %s in inspect output", l)
			}
		}
	}

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("inspect"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("inspect"),
		e2e.WithArgs([]string{"--labels", imagePath}...),
		e2e.ExpectExit(0, verifyOutput),
	)
}

//nolint:dupl
func (c ctx) testDockerCMD(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "docker-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("while getting $HOME: %s", err)
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs(imagePath, "docker://ghcr.io/apptainer/docker-cmd"),
		e2e.ExpectExit(0),
	)

	tests := []struct {
		name         string
		args         []string
		noeval       bool
		expectOutput string
	}{
		// Apptainer historic behavior (without --no-eval)
		// These do not all match Docker, due to evaluation, consumption of quoting.
		{
			name:         "default",
			args:         []string{},
			noeval:       false,
			expectOutput: `CMD 'quotes' "quotes" $DOLLAR s p a c e s`,
		},
		{
			name:         "override",
			args:         []string{"echo", "test"},
			noeval:       false,
			expectOutput: `test`,
		},
		{
			name:         "override env var",
			args:         []string{"echo", "$HOME"},
			noeval:       false,
			expectOutput: home,
		},
		// This looks very wrong, but is historic behavior
		{
			name:         "override sh echo",
			args:         []string{"sh", "-c", `echo "hello there"`},
			noeval:       false,
			expectOutput: "hello",
		},
		// Docker/OCI behavior (with --no-eval)
		{
			name:         "no-eval/default",
			args:         []string{},
			noeval:       true,
			expectOutput: `CMD 'quotes' "quotes" $DOLLAR s p a c e s`,
		},
		{
			name:         "no-eval/override",
			args:         []string{"echo", "test"},
			noeval:       true,
			expectOutput: `test`,
		},
		{
			name:         "no-eval/override env var",
			noeval:       true,
			args:         []string{"echo", "$HOME"},
			expectOutput: "$HOME",
		},
		{
			name:         "no-eval/override sh echo",
			noeval:       true,
			args:         []string{"sh", "-c", `echo "hello there"`},
			expectOutput: "hello there",
		},
	}

	for _, tt := range tests {
		cmdArgs := []string{}
		if tt.noeval {
			cmdArgs = append(cmdArgs, "--no-eval")
		}
		cmdArgs = append(cmdArgs, imagePath)
		cmdArgs = append(cmdArgs, tt.args...)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("run"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(0,
				e2e.ExpectOutput(e2e.ExactMatch, tt.expectOutput),
			),
		)
	}
}

//nolint:dupl
func (c ctx) testDockerENTRYPOINT(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "docker-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("while getting $HOME: %s", err)
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs(imagePath, "docker://ghcr.io/apptainer/docker-entrypoint"),
		e2e.ExpectExit(0),
	)

	tests := []struct {
		name         string
		args         []string
		noeval       bool
		expectOutput string
	}{
		// Apptainer historic behavior (without --no-eval)
		// These do not all match Docker, due to evaluation, consumption of quoting.
		{
			name:         "default",
			args:         []string{},
			noeval:       false,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s`,
		},
		{
			name:         "override",
			args:         []string{"echo", "test"},
			noeval:       false,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo test`,
		},
		{
			name:         "override env var",
			args:         []string{"echo", "$HOME"},
			noeval:       false,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo ` + home,
		},
		// Docker/OCI behavior (with --no-eval)
		{
			name:         "no-eval/default",
			args:         []string{},
			noeval:       true,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s`,
		},
		{
			name:         "no-eval/override",
			args:         []string{"echo", "test"},
			noeval:       true,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo test`,
		},
		{
			name:         "no-eval/override env var",
			noeval:       true,
			args:         []string{"echo", "$HOME"},
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo $HOME`,
		},
	}

	for _, tt := range tests {
		cmdArgs := []string{}
		if tt.noeval {
			cmdArgs = append(cmdArgs, "--no-eval")
		}
		cmdArgs = append(cmdArgs, imagePath)
		cmdArgs = append(cmdArgs, tt.args...)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("run"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(0,
				e2e.ExpectOutput(e2e.ExactMatch, tt.expectOutput),
			),
		)
	}
}

//nolint:dupl
func (c ctx) testDockerCMDENTRYPOINT(t *testing.T) {
	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "docker-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
	imagePath := filepath.Join(imageDir, "container")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("while getting $HOME: %s", err)
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull"),
		e2e.WithArgs(imagePath, "docker://ghcr.io/apptainer/docker-cmd-entrypoint"),
		e2e.ExpectExit(0),
	)

	tests := []struct {
		name         string
		args         []string
		noeval       bool
		expectOutput string
	}{
		// Apptainer historic behavior (without --no-eval)
		// These do not all match Docker, due to evaluation, consumption of quoting.
		{
			name:         "default",
			args:         []string{},
			noeval:       false,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s CMD 'quotes' "quotes" $DOLLAR s p a c e s`,
		},
		{
			name:         "override",
			args:         []string{"echo", "test"},
			noeval:       false,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo test`,
		},
		{
			name:         "override env var",
			args:         []string{"echo", "$HOME"},
			noeval:       false,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo ` + home,
		},
		// Docker/OCI behavior (with --no-eval)
		{
			name:         "no-eval/default",
			args:         []string{},
			noeval:       true,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s CMD 'quotes' "quotes" $DOLLAR s p a c e s`,
		},
		{
			name:         "no-eval/override",
			args:         []string{"echo", "test"},
			noeval:       true,
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo test`,
		},
		{
			name:         "no-eval/override env var",
			noeval:       true,
			args:         []string{"echo", "$HOME"},
			expectOutput: `ENTRYPOINT 'quotes' "quotes" $DOLLAR s p a c e s echo $HOME`,
		},
	}

	for _, tt := range tests {
		cmdArgs := []string{}
		if tt.noeval {
			cmdArgs = append(cmdArgs, "--no-eval")
		}
		cmdArgs = append(cmdArgs, imagePath)
		cmdArgs = append(cmdArgs, tt.args...)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("run"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(0,
				e2e.ExpectOutput(e2e.ExactMatch, tt.expectOutput),
			),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		// Run most docker:// source tests sequentially amongst themselves, so we
		// don't hit DockerHub massively in parallel, and we benefit from
		// caching as the same images are used frequently.
		"ordered": func(t *testing.T) {
			t.Run("AUFS", c.testDockerAUFS)
			t.Run("def file", c.testDockerDefFile)
			t.Run("permissions", c.testDockerPermissions)
			t.Run("pulls", c.testDockerPulls)
			t.Run("whiteout symlink", c.testDockerWhiteoutSymlink)
			t.Run("labels", c.testDockerLabels)
			t.Run("cmd", c.testDockerCMD)
			t.Run("entrypoint", c.testDockerENTRYPOINT)
			t.Run("cmdentrypoint", c.testDockerCMDENTRYPOINT)
			t.Run("cmd quotes", c.testDockerCMDQuotes)
			// Regressions
			t.Run("issue 4524", c.issue4524)
		},
		// Tests that are especially slow, or run against a local docker
		// registry, can be run in parallel, with `--disable-cache` used within
		// them to avoid docker caching concurrency issues.
		"docker host": c.testDockerHost,
		"registry":    c.testDockerRegistry,
		// Regressions
		"issue 4943": c.issue4943,
		"issue 5172": c.issue5172,
		"issue 274":  c.issue274, // https://github.com/sylabs/singularity/issues/274
	}
}
