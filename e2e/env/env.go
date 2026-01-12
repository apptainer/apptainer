// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// This test sets apptainer image specific environment variables and
// verifies that they are properly set.

package apptainerenv

import (
	"fmt"
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
	e2e.EnsureImage(t, c.env)
	// Apptainer defines a path by default. See apptainerware/apptainer/etc/init.
	defaultImage := e2e.BusyboxSIF(t)
	// This image sets a custom path.
	// See e2e/testdata/Apptainer
	customImage := c.env.ImagePath
	customPath := defaultPath + ":/go/bin:/usr/local/go/bin"

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
	// Apptainer defines a path by default. See apptainerware/apptainer/etc/init.
	defaultImage := e2e.BusyboxSIF(t)
	// This image sets a custom path.
	// See e2e/testdata/Apptainer
	customImage := c.env.ImagePath
	customPath := defaultPath + ":/go/bin:/usr/local/go/bin"

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
			image:    defaultImage,
			matchEnv: "PATH",
			matchVal: defaultPath,
		},
		{
			name:     "DefaultPathOverride",
			image:    defaultImage,
			envOpt:   []string{"PATH=/"},
			matchEnv: "PATH",
			matchVal: "/",
		},
		{
			name:     "AppendDefaultPath",
			image:    defaultImage,
			envOpt:   []string{"APPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: defaultPath + ":/foo",
		},
		{
			name:     "PrependDefaultPath",
			image:    defaultImage,
			envOpt:   []string{"PREPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: "/foo:" + defaultPath,
		},
		{
			name:     "DefaultPathImage",
			image:    customImage,
			matchEnv: "PATH",
			matchVal: customPath,
		},
		{
			name:     "DefaultPathTestImageOverride",
			image:    customImage,
			envOpt:   []string{"PATH=/"},
			matchEnv: "PATH",
			matchVal: "/",
		},
		{
			name:     "AppendDefaultPathTestImage",
			image:    customImage,
			envOpt:   []string{"APPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: customPath + ":/foo",
		},
		{
			name:     "AppendLiteralDefaultPathTestImage",
			image:    customImage,
			envOpt:   []string{"PATH=$PATH:/foo"},
			matchEnv: "PATH",
			matchVal: customPath + ":/foo",
		},
		{
			name:     "PrependDefaultPathTestImage",
			image:    customImage,
			envOpt:   []string{"PREPEND_PATH=/foo"},
			matchEnv: "PATH",
			matchVal: "/foo:" + customPath,
		},
		{
			name:     "PrependLiteralDefaultPathTestImage",
			image:    customImage,
			envOpt:   []string{"PATH=/foo:$PATH"},
			matchEnv: "PATH",
			matchVal: "/foo:" + customPath,
		},
		{
			name:     "TestImageCgoEnabledDefault",
			image:    customImage,
			matchEnv: "CGO_ENABLED",
			matchVal: "0",
		},
		{
			name:     "TestImageCgoEnabledOverride",
			image:    customImage,
			envOpt:   []string{"CGO_ENABLED=1"},
			matchEnv: "CGO_ENABLED",
			matchVal: "1",
		},
		{
			name:     "TestImageCgoEnabledOverride_KO",
			image:    customImage,
			hostEnv:  []string{"CGO_ENABLED=1"},
			matchEnv: "CGO_ENABLED",
			matchVal: "0",
		},
		{
			name:     "TestImageCgoEnabledOverrideFromEnv",
			image:    customImage,
			hostEnv:  []string{"APPTAINERENV_CGO_ENABLED=1"},
			matchEnv: "CGO_ENABLED",
			matchVal: "1",
		},
		{
			name:     "TestImageCgoEnabledOverrideEnvOptionPrecedence",
			image:    customImage,
			hostEnv:  []string{"APPTAINERENV_CGO_ENABLED=1"},
			envOpt:   []string{"CGO_ENABLED=2"},
			matchEnv: "CGO_ENABLED",
			matchVal: "2",
		},
		{
			name:     "TestImageCgoEnabledOverrideEmpty",
			image:    customImage,
			envOpt:   []string{"CGO_ENABLED="},
			matchEnv: "CGO_ENABLED",
			matchVal: "",
		},
		{
			name:     "TestImageOverrideHost",
			image:    customImage,
			hostEnv:  []string{"FOO=bar"},
			envOpt:   []string{"FOO=foo"},
			matchEnv: "FOO",
			matchVal: "foo",
		},
		{
			name:     "TestMultiLine",
			image:    customImage,
			hostEnv:  []string{"MULTI=Hello\nWorld"},
			matchEnv: "MULTI",
			matchVal: "Hello\nWorld",
		},
		{
			name:     "TestEscapedNewline",
			image:    customImage,
			hostEnv:  []string{"ESCAPED=Hello\\nWorld"},
			matchEnv: "ESCAPED",
			matchVal: "Hello\\nWorld",
		},
		{
			name:  "TestInvalidKey",
			image: customImage,
			// We try to set an invalid env var... and make sure
			// we have no error output from the interpreter as it
			// should be ignored, not passed into the container.
			hostEnv:  []string{"BASH_FUNC_ml%%=TEST"},
			matchEnv: "BASH_FUNC_ml%%",
			matchVal: "",
		},
		{
			name:     "TestDefaultLdLibraryPath",
			image:    customImage,
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: apptainerLibs,
		},
		{
			name:     "TestCustomTrailingCommaPath",
			image:    customImage,
			envOpt:   []string{"LD_LIBRARY_PATH=/foo,"},
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo,:" + apptainerLibs,
		},
		{
			name:     "TestCustomLdLibraryPath",
			image:    customImage,
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

func (c ctx) apptainerNoEnvOption(t *testing.T) {
	e2e.EnsureImage(t, c.env)
	image := e2e.BusyboxSIF(t)

	tests := []struct {
		name     string
		noEnvOpt []string
		hostEnv  []string
		matchEnv string
		matchVal string
	}{
		{
			name:     "NormalImport",
			hostEnv:  []string{"FOO=bar", "BAR=foo"},
			matchEnv: "FOO",
			matchVal: "bar",
		},
		{
			name:     "NoEnvAlone",
			noEnvOpt: []string{"FOO"},
			hostEnv:  []string{"FOO=bar", "BAR=foo"},
			matchEnv: "FOO",
			matchVal: "",
		},
		{
			name:     "NoEnvSecond",
			noEnvOpt: []string{"BAR", "FOO"},
			hostEnv:  []string{"FOO=bar", "BAR=foo"},
			matchEnv: "FOO",
			matchVal: "",
		},
		{
			name: "NoEnvViaEnv",
			hostEnv: []string{
				"FOO=bar",
				"BAR=foo",
				"APPTAINER_NOENV=FOO,BAR",
			},
			matchEnv: "FOO",
			matchVal: "",
		},
		{
			name:     "OverridePermitted",
			noEnvOpt: []string{"BAR", "FOO"},
			hostEnv: []string{
				"FOO=bar",
				"BAR=foo",
				"APPTAINERENV_FOO=Bar",
			},
			matchEnv: "FOO",
			matchVal: "Bar",
		},
		{
			name:     "OverrideNotPermitted",
			noEnvOpt: []string{"APPTAINER_APPNAME"},
			hostEnv: []string{
				"APPTAINERENV_APPTAINER_APPNAME=foo",
			},
			matchEnv: "APPTAINER_APPNAME",
			matchVal: "",
		},
	}

	for _, tt := range tests {
		args := make([]string, 0)
		if tt.noEnvOpt != nil {
			args = append(args, "--no-env", strings.Join(tt.noEnvOpt, ","))
		}
		args = append(args, image, "/bin/sh", "-c", "echo \"${"+tt.matchEnv+"}\"")
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

	tests := []struct {
		name     string
		image    string
		envFiles []string
		envOpt   []string
		hostEnv  []string
		matchEnv string
		matchVal string
	}{
		{
			name:     "DefaultPathOverride",
			image:    c.env.ImagePath,
			envFiles: []string{"PATH=/"},
			matchEnv: "PATH",
			matchVal: "/",
		},
		{
			name:     "DefaultPathOverrideEnvOptionPrecedence",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=/etc"},
			envFiles: []string{"PATH=/"},
			matchEnv: "PATH",
			matchVal: "/etc",
		},
		{
			name:     "DefaultPathOverrideEnvFileOptionPrecedence",
			image:    c.env.ImagePath,
			envFiles: []string{"PATH=/", "PATH=/etc"},
			matchEnv: "PATH",
			matchVal: "/etc",
		},
		{
			name:     "DefaultPathOverrideEnvAndEnvFileOptionPrecedence",
			image:    c.env.ImagePath,
			envOpt:   []string{"PATH=/etc"},
			envFiles: []string{"PATH=/", "PATH=/foo"},
			matchEnv: "PATH",
			matchVal: "/etc",
		},
		{
			name:     "AppendDefaultPath",
			image:    c.env.ImagePath,
			envFiles: []string{"APPEND_PATH=/"},
			matchEnv: "PATH",
			matchVal: imageDefaultPath + ":/",
		},
		{
			name:     "AppendLiteralDefaultPath",
			image:    c.env.ImagePath,
			envFiles: []string{`PATH="\$PATH:/"`},
			matchEnv: "PATH",
			matchVal: imageDefaultPath + ":/",
		},
		{
			name:     "PrependLiteralDefaultPath",
			image:    c.env.ImagePath,
			envFiles: []string{`PATH="/:\$PATH"`},
			matchEnv: "PATH",
			matchVal: "/:" + imageDefaultPath,
		},
		{
			name:     "PrependDefaultPath",
			image:    c.env.ImagePath,
			envFiles: []string{"PREPEND_PATH=/"},
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
			envFiles: []string{"LD_LIBRARY_PATH=/foo"},
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo:" + apptainerLibs,
		},
		{
			name:     "CustomTrailingCommaPath",
			image:    c.env.ImagePath,
			envFiles: []string{"LD_LIBRARY_PATH=/foo,"},
			matchEnv: "LD_LIBRARY_PATH",
			matchVal: "/foo,:" + apptainerLibs,
		},
	}

	for _, tt := range tests {
		args := make([]string, 0)
		if tt.envOpt != nil {
			args = append(args, "--env", strings.Join(tt.envOpt, ","))
		}
		if len(tt.envFiles) > 0 {
			for i, envFile := range tt.envFiles {
				filename := fmt.Sprint(p, i)
				os.WriteFile(filename, []byte(envFile), 0o644)
				args = append(args, "--env-file", filename)
			}
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

// apptainerRuntimeTimetutEnv tests the new apptainer run/instance option `--runscript-timeout`
func (c ctx) apptainerRuntimeTimeoutEnv(t *testing.T) {
	e2e.EnsureImage(t, c.env)
	// test script
	dir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "runscript-", "")
	defer cleanup(t)
	path := filepath.Join(dir, "sleep.sh")
	err := os.WriteFile(path, []byte("sleep 2s"), 0o755)
	if err != nil {
		t.Fatalf("unable to create timeout script, err: %s", err)
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs("-B", fmt.Sprintf("%s:/.singularity.d/env/91-slow_env.sh", path), "--runscript-timeout", "1s", c.env.ImagePath, "true"),
		e2e.ExpectExit(255, e2e.ExpectError(e2e.ContainMatch, "FATAL:   command \"sleep 2s\" was killed after 1s timeout")),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"environment manipulation": c.apptainerEnv,
		"environment option":       c.apptainerEnvOption,
		"--no-env option":          c.apptainerNoEnvOption,
		"environment file":         c.apptainerEnvFile,
		"env eval":                 c.apptainerEnvEval,
		"issue 5057":               c.issue5057,                  // https://github.com/apptainer/singularity/issues/5057
		"issue 5426":               c.issue5426,                  // https://github.com/apptainer/singularity/issues/5426
		"issue 43":                 c.issue43,                    // https://github.com/sylabs/singularity/issues/43
		"issue 1263":               c.issue1263,                  // https://github.com/sylabs/singularity/issues/1263
		"runscript timeout":        c.apptainerRuntimeTimeoutEnv, // https://github.com/apptainer/apptainer/issues/1595
	}
}
