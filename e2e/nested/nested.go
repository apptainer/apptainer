// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package nested

import (
	"os"
	"os/exec"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
)

type ctx struct {
	env e2e.TestEnv
}

//nolint:dupl
func (c ctx) docker(t *testing.T) {
	// Execution using privileged Docker. Root in outer Docker container.
	c.nestedContainerTest(t, "docker", "apptainer-e2e-docker-nested")
}

func (c ctx) nestedContainerTest(t *testing.T, prog, ref string) {
	require.Command(t, prog)
	e2e.EnsureORASImage(t, c.env)

	// Temporary homedir for docker/podman commands, so they don't create
	// a ~/.docker that may interfere elsewhere.
	tmpHome, cleanupHome := e2e.MakeTempDir(t, c.env.TestDir, "nested-"+prog+"-", "")
	t.Cleanup(func() { e2e.Privileged(cleanupHome)(t) })

	dockerFile := "testdata/Dockerfile.nested"

	containerBuild(t, prog, dockerFile, ref, "../", c.env.DebianImageSource, tmpHome)
	defer containerRMI(t, prog, ref, tmpHome)

	containerRun(t, prog, "version", ref, tmpHome)
	containerRun(t, prog, "exec", ref, tmpHome, "exec", c.env.OrasTestImage, "/bin/true")
	containerRun(t, prog, "execUserNS", ref, tmpHome, "exec", "-u", c.env.OrasTestImage, "/bin/true")
	containerRun(t, prog, "buildSIF", ref, tmpHome, "build", "test.sif", "/e2e/testdata/Apptainer")
}

func containerBuild(t *testing.T, prog, dockerFile, ref, contextPath, baseImage, homeDir string) {
	t.Run("build/"+ref, e2e.Privileged(func(t *testing.T) {
		cmd := exec.Command(prog, "build", "--network=host",
			"--build-arg", "BASEIMAGE="+baseImage,
			"-t", ref, "-f", dockerFile, contextPath)
		cmd.Env = append(cmd.Env, "HOME="+homeDir)
		out, err := cmd.CombinedOutput()
		t.Log(cmd.Args)
		if err != nil {
			t.Fatalf("Failed building %s container.\n%s: %s", prog, err, string(out))
		}
	}))
}

func containerRMI(t *testing.T, prog, ref, homeDir string) {
	t.Run("rmi/"+ref, e2e.Privileged(func(t *testing.T) {
		cmd := exec.Command(prog, "rmi", ref)
		cmd.Env = append(cmd.Env, "HOME="+homeDir)
		out, err := cmd.CombinedOutput()
		t.Log(cmd.Args)
		if err != nil {
			t.Fatalf("Failed removing %s container.\n%s: %s", prog, err, string(out))
		}
	}))
}

func containerRun(t *testing.T, prog, name, ref, homeDir string, args ...string) { //nolint:unparam
	t.Run(name, e2e.Privileged(func(t *testing.T) {
		cwd, _ := os.Getwd()
		cmdArgs := []string{
			"run", "-i", "--rm", "--privileged", "--network=host",
			"-v", "/usr/local:/usr/local",
			"-v", cwd + ":/e2e",
			ref,
		}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command(prog, cmdArgs...)
		cmd.Env = append(cmd.Env, "HOME="+homeDir)
		out, err := cmd.CombinedOutput()
		t.Log(cmd.Args)
		if err != nil {
			t.Fatalf("Failed running %s container.\n%s: %s", prog, err, string(out))
		}
	}))
}

//nolint:dupl
func (c ctx) podman(t *testing.T) {
	// Rootless podman - fake userns root in outer podman container.
	c.nestedContainerTest(t, "podman", "localhost/apptainer-e2e-podman-nested")
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"Docker": c.docker,
		"Podman": c.podman,
	}
}
