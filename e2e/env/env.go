// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// This test sets apptainer image specific environment variables and
// verifies that they are properly set.

package apptainerenv

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

const (
	defaultPath   = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	apptainerLibs = "/.singularity.d/libs"
)

func (c ctx) apptainerEnv(t *testing.T) {
	// use a cache to not download images over and over
	imgCacheDir, cleanCache := e2e.MakeCacheDir(t, c.env.TestDir)
	defer cleanCache(t)
	c.env.ImgCacheDir = imgCacheDir

	// Apptainer defines a path by default. See apptainerware/apptainer/etc/init.
	defaultImage := "docker://alpine:3.8"

	// This image sets a custom path.
	customImage := "docker://ghcr.io/apptainer/lolcow"
	customPath := "/usr/games:" + defaultPath

	// Append or prepend this path.
	partialPath := "/foo"

	// Overwrite the path with this one.
	overwrittenPath := "/usr/bin:/bin"

	// A path with a trailing comma
	trailingCommaPath := "/usr/bin:/bin,"

	tests := []struct {
		name  string
		image string
		path  string
		env   []string
	}{
		{
			name:  "DefaultPath",
			image: defaultImage,
			path:  defaultPath,
			env:   []string{},
		},
		{
			name:  "CustomPath",
			image: customImage,
			path:  customPath,
			env:   []string{},
		},
		{
			name:  "AppendToDefaultPath",
			image: defaultImage,
			path:  defaultPath + ":" + partialPath,
			env:   []string{"APPTAINERENV_APPEND_PATH=/foo"},
		},
		{
			name:  "AppendToCustomPath",
			image: customImage,
			path:  customPath + ":" + partialPath,
			env:   []string{"APPTAINERENV_APPEND_PATH=/foo"},
		},
		{
			name:  "PrependToDefaultPath",
			image: defaultImage,
			path:  partialPath + ":" + defaultPath,
			env:   []string{"APPTAINERENV_PREPEND_PATH=/foo"},
		},
		{
			name:  "PrependToCustomPath",
			image: customImage,
			path:  partialPath + ":" + customPath,
			env:   []string{"APPTAINERENV_PREPEND_PATH=/foo"},
		},
		{
			name:  "OverwriteDefaultPath",
			image: defaultImage,
			path:  overwrittenPath,
			env:   []string{"APPTAINERENV_PATH=" + overwrittenPath},
		},
		{
			name:  "OverwriteCustomPath",
			image: customImage,
			path:  overwrittenPath,
			env:   []string{"APPTAINERENV_PATH=" + overwrittenPath},
		},
		{
			name:  "OverwriteTrailingCommaPath",
			image: defaultImage,
			path:  trailingCommaPath,
			env:   []string{"APPTAINERENV_PATH=" + trailingCommaPath},
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithEnv(tt.env),
			e2e.WithArgs(tt.image, "/bin/sh", "-c", "echo $PATH"),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(e2e.ExactMatch, tt.path),
			),
		)
	}
}

func (c ctx) apptainerEnvOption(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	imageDefaultPath := defaultPath + ":/go/bin:/usr/local/go/bin"

	// use a cache to not download images over and over
	imgCacheDir, cleanCache := e2e.MakeCacheDir(t, c.env.TestDir)
	defer cleanCache(t)
	c.env.ImgCacheDir = imgCacheDir

	tests := []struct {
		name     string
		image    string
		envOpt   []string
		hostEnv  []string
		matchEnv string
		matchVal string
	}{
		{
			name:     "DefaultPath",
			image:    "docker://alpine:3.8",
			matchEnv: "PATH",
			matchVal: defaultPath,
		},
		{
			name:     "DefaultPathOverride",
			image:    "docker://alpine:3.8",
			envOpt:   []string{"PATH=/"},
			matchEnv: "PATH",
			matchVal: "/",
		},
		{
			name:     "AppendDefaultPath",
			image:    "docker://alpine:3.8",
			envOpt:   []string{"APPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: defaultPath + ":/foo",
		},
		{
			name:     "PrependDefaultPath",
			image:    "docker://alpine:3.8",
			envOpt:   []string{"PREPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: "/foo:" + defaultPath,
		},
		{
			name:     "DockerImage",
			image:    "docker://ghcr.io/apptainer/lolcow",
			matchEnv: "LC_ALL",
			matchVal: "C",
		},
		{
			name:     "DockerImageOverride",
			image:    "docker://ghcr.io/apptainer/lolcow",
			envOpt:   []string{"LC_ALL=foo"},
			matchEnv: "LC_ALL",
			matchVal: "foo",
		},
		{
			name:     "DefaultPathTestImage",
			image:    c.env.ImagePath,
			matchEnv: "PATH",
			matchVal: imageDefaultPath,
		},
		{
			name:     "DefaultPathTestImageOverride",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=/"},
			matchEnv: "PATH",
			matchVal: "/",
		},
		{
			name:     "AppendDefaultPathTestImage",
			image:    c.env.ImagePath,
			envOpt:   []string{"APPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: imageDefaultPath + ":/foo",
		},
		{
			name:     "AppendLiteralDefaultPathTestImage",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=$PATH:/foo"},
			matchEnv: "PATH",
			matchVal: imageDefaultPath + ":/foo",
		},
		{
			name:     "PrependDefaultPathTestImage",
			image:    c.env.ImagePath,
			envOpt:   []string{"PREPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: "/foo:" + imageDefaultPath,
		},
		{
			name:     "PrependLiteralDefaultPathTestImage",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=/foo:$PATH"},
			matchEnv: "PATH",
			matchVal: "/foo:" + imageDefaultPath,
		},
		{
			name:     "TestImageCgoEnabledDefault",
			image:    c.env.ImagePath,
			matchEnv: "CGO_ENABLED",
			matchVal: "0",
		},
		{
			name:     "TestImageCgoEnabledOverride",
			image:    c.env.ImagePath,
			envOpt:   []string{"CGO_ENABLED=1"},
			matchEnv: "CGO_ENABLED",
			matchVal: "1",
		},
		{
			name:     "TestImageCgoEnabledOverride_KO",
			image:    c.env.ImagePath,
			hostEnv:  []string{"CGO_ENABLED=1"},
			matchEnv: "CGO_ENABLED",
			matchVal: "0",
		},
		{
			name:     "TestImageCgoEnabledOverrideFromEnv",
			image:    c.env.ImagePath,
			hostEnv:  []string{"APPTAINERENV_CGO_ENABLED=1"},
			matchEnv: "CGO_ENABLED",
			matchVal: "1",
		},
		{
			name:     "TestImageCgoEnabledOverrideEnvOptionPrecedence",
			image:    c.env.ImagePath,
			hostEnv:  []string{"APPTAINERENV_CGO_ENABLED=1"},
			envOpt:   []string{"CGO_ENABLED=2"},
			matchEnv: "CGO_ENABLED",
			matchVal: "2",
		},
		{
			name:     "TestImageCgoEnabledOverrideEmpty",
			image:    c.env.ImagePath,
			envOpt:   []string{"CGO_ENABLED="},
			matchEnv: "CGO_ENABLED",
			matchVal: "",
		},
		{
			name:     "TestImageOverrideHost",
			image:    c.env.ImagePath,
			hostEnv:  []string{"FOO=bar"},
			envOpt:   []string{"FOO=foo"},
			matchEnv: "FOO",
			matchVal: "foo",
		},
		{
			name:     "TestMultiLine",
			image:    c.env.ImagePath,
			hostEnv:  []string{"MULTI=Hello\nWorld"},
			matchEnv: "MULTI",
			matchVal: "Hello\nWorld",
		},
		{
			name:     "TestEscapedNewline",
			image:    c.env.ImagePath,
			hostEnv:  []string{"ESCAPED=Hello\\nWorld"},
			matchEnv: "ESCAPED",
			matchVal: "Hello\\nWorld",
		},
		{
			name:  "TestInvalidKey",
			image: c.env.ImagePath,
			// We try to set an invalid env var... and make sure
			// we have no error output from the interpreter as it
			// should be ignored, not passed into the container.
			hostEnv:  []string{"BASH_FUNC_ml%%=TEST"},
			matchEnv: "BASH_FUNC_ml%%",
			matchVal: "",
		},
		{
			name:     "TestDefaultLdLibraryPath",
			image:    c.env.ImagePath,
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: apptainerLibs,
		},
		{
			name:     "TestCustomTrailingCommaPath",
			image:    c.env.ImagePath,
			envOpt:   []string{"LD_LIBRARY_PATH=/foo,"},
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo,:" + apptainerLibs,
		},
		{
			name:     "TestCustomLdLibraryPath",
			image:    c.env.ImagePath,
			envOpt:   []string{"LD_LIBRARY_PATH=/foo"},
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo:" + apptainerLibs,
		},
	}

	for _, tt := range tests {
		args := make([]string, 0)
		if tt.envOpt != nil {
			args = append(args, "--env", strings.Join(tt.envOpt, ","))
		}
		args = append(args, tt.image, "/bin/sh", "-c", "echo \"${"+tt.matchEnv+"}\"")
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithEnv(tt.hostEnv),
			e2e.WithArgs(args...),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(e2e.ExactMatch, tt.matchVal),
			),
		)
	}
}

func (c ctx) apptainerEnvFile(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	imageDefaultPath := defaultPath + ":/go/bin:/usr/local/go/bin"

	dir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "envfile-", "")
	defer cleanup(t)
	p := filepath.Join(dir, "env.file")

	// use a cache to not download images over and over
	imgCacheDir, cleanCache := e2e.MakeCacheDir(t, c.env.TestDir)
	defer cleanCache(t)
	c.env.ImgCacheDir = imgCacheDir

	tests := []struct {
		name     string
		image    string
		envFile  string
		envOpt   []string
		hostEnv  []string
		matchEnv string
		matchVal string
	}{
		{
			name:     "DefaultPathOverride",
			image:    c.env.ImagePath,
			envFile:  "PATH=/",
			matchEnv: "PATH",
			matchVal: "/",
		},
		{
			name:     "DefaultPathOverrideEnvOptionPrecedence",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=/etc"},
			envFile:  "PATH=/",
			matchEnv: "PATH",
			matchVal: "/etc",
		},
		{
			name:     "DefaultPathOverrideEnvOptionPrecedence",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=/etc"},
			envFile:  "PATH=/",
			matchEnv: "PATH",
			matchVal: "/etc",
		},
		{
			name:     "AppendDefaultPath",
			image:    c.env.ImagePath,
			envFile:  "APPEND_PATH=/",
			matchEnv: "PATH",
			matchVal: imageDefaultPath + ":/",
		},
		{
			name:     "AppendLiteralDefaultPath",
			image:    c.env.ImagePath,
			envFile:  `PATH="\$PATH:/"`,
			matchEnv: "PATH",
			matchVal: imageDefaultPath + ":/",
		},
		{
			name:     "PrependLiteralDefaultPath",
			image:    c.env.ImagePath,
			envFile:  `PATH="/:\$PATH"`,
			matchEnv: "PATH",
			matchVal: "/:" + imageDefaultPath,
		},
		{
			name:     "PrependDefaultPath",
			image:    c.env.ImagePath,
			envFile:  "PREPEND_PATH=/",
			matchEnv: "PATH",
			matchVal: "/:" + imageDefaultPath,
		},
		{
			name:     "DefaultLdLibraryPath",
			image:    c.env.ImagePath,
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: apptainerLibs,
		},
		{
			name:     "CustomLdLibraryPath",
			image:    c.env.ImagePath,
			envFile:  "LD_LIBRARY_PATH=/foo",
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo:" + apptainerLibs,
		},
		{
			name:     "CustomTrailingCommaPath",
			image:    c.env.ImagePath,
			envFile:  "LD_LIBRARY_PATH=/foo,",
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo,:" + apptainerLibs,
		},
	}

	for _, tt := range tests {
		args := make([]string, 0)
		if tt.envOpt != nil {
			args = append(args, "--env", strings.Join(tt.envOpt, ","))
		}
		if tt.envFile != "" {
			ioutil.WriteFile(p, []byte(tt.envFile), 0o644)
			args = append(args, "--env-file", p)
		}
		args = append(args, tt.image, "/bin/sh", "-c", "echo $"+tt.matchEnv)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithEnv(tt.hostEnv),
			e2e.WithArgs(args...),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(e2e.ExactMatch, tt.matchVal),
			),
		)
	}
}

// Check for evaluation of env vars with / without `--no-eval`. By default,
// Apptainer will evaluate the value of injected env vars when sourcing the
// shell script that injects them. With --no-eval it should match Docker, with
// no evaluation:
//
//	WHO='$(id -u)' docker run -it --env WHO --rm alpine sh -c 'echo $WHO'
//	$(id -u)
func (c ctx) apptainerEnvEval(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	testArgs := []string{"/bin/sh", "-c", "echo $WHO"}

	tests := []struct {
		name         string
		env          []string
		args         []string
		noeval       bool
		expectOutput string
	}{
		// Apptainer historic behavior (without --no-eval)
		{
			name:         "no env",
			args:         testArgs,
			env:          []string{},
			noeval:       false,
			expectOutput: "",
		},
		{
			name:         "string env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=ME"},
			noeval:       false,
			expectOutput: "ME",
		},
		{
			name:         "env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=$UID"},
			noeval:       false,
			expectOutput: strconv.Itoa(os.Getuid()),
		},
		{
			name:         "double quoted env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\"$UID\""},
			noeval:       false,
			expectOutput: "\"" + strconv.Itoa(os.Getuid()) + "\"",
		},
		{
			name:         "single quoted env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO='$UID'"},
			noeval:       false,
			expectOutput: "'" + strconv.Itoa(os.Getuid()) + "'",
		},
		{
			name:         "escaped env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\\$UID"},
			noeval:       false,
			expectOutput: "$UID",
		},
		{
			name:         "subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=$(id -u)"},
			noeval:       false,
			expectOutput: strconv.Itoa(os.Getuid()),
		},
		{
			name:         "double quoted subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\"$(id -u)\""},
			noeval:       false,
			expectOutput: "\"" + strconv.Itoa(os.Getuid()) + "\"",
		},
		{
			name:         "single quoted subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO='$(id -u)'"},
			noeval:       false,
			expectOutput: "'" + strconv.Itoa(os.Getuid()) + "'",
		},
		{
			name:         "escaped subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\\$(id -u)"},
			noeval:       false,
			expectOutput: "$(id -u)",
		},
		// Docker/OCI behavior (with --no-eval)
		{
			name:         "no-eval/no env",
			args:         testArgs,
			env:          []string{},
			noeval:       false,
			expectOutput: "",
		},
		{
			name:         "no-eval/string env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=ME"},
			noeval:       false,
			expectOutput: "ME",
		},
		{
			name:         "no-eval/env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=$UID"},
			noeval:       true,
			expectOutput: "$UID",
		},
		{
			name:         "no-eval/double quoted env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\"$UID\""},
			noeval:       true,
			expectOutput: "\"$UID\"",
		},
		{
			name:         "no-eval/single quoted env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO='$UID'"},
			noeval:       true,
			expectOutput: "'$UID'",
		},
		{
			name:         "no-eval/escaped env var",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\\$UID"},
			noeval:       true,
			expectOutput: "\\$UID",
		},
		{
			name:         "no-eval/subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=$(id -u)"},
			noeval:       true,
			expectOutput: "$(id -u)",
		},
		{
			name:         "no-eval/double quoted subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\"$(id -u)\""},
			noeval:       true,
			expectOutput: "\"$(id -u)\"",
		},
		{
			name:         "no-eval/single quoted subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO='$(id -u)'"},
			noeval:       true,
			expectOutput: "'$(id -u)'",
		},
		{
			name:         "no-eval/escaped subshell env",
			args:         testArgs,
			env:          []string{"APPTAINERENV_WHO=\\$(id -u)"},
			noeval:       true,
			expectOutput: "\\$(id -u)",
		},
	}

	for _, tt := range tests {
		cmdArgs := []string{}
		if tt.noeval {
			cmdArgs = append(cmdArgs, "--no-eval")
		}
		cmdArgs = append(cmdArgs, c.env.ImagePath)
		cmdArgs = append(cmdArgs, tt.args...)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithEnv(tt.env),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
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
		"environment manipulation": c.apptainerEnv,
		"environment option":       c.apptainerEnvOption,
		"environment file":         c.apptainerEnvFile,
		"env eval":                 c.apptainerEnvEval,
		"issue 5057":               c.issue5057, // https://github.com/apptainer/singularity/issues/5057
		"issue 5426":               c.issue5426, // https://github.com/apptainer/singularity/issues/5426
		"issue 43":                 c.issue43,   // https://github.com/sylabs/singularity/issues/43
		"issue 274":                c.issue274,  // https://github.com/sylabs/singularity/issues/274
	}
}
