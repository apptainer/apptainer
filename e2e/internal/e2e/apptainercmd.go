// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/pkg/errors"
)

// ApptainerCmdResultOp is a function type executed
// by ExpectExit to process and test execution result.
type ApptainerCmdResultOp func(*testing.T, *ApptainerCmdResult)

// ApptainerCmdResult holds the result for an Apptainer command
// execution test.
type ApptainerCmdResult struct {
	Stdout  []byte
	Stderr  []byte
	FullCmd string
}

// MatchType defines the type of match for ExpectOutput and ExpectError
// functions
type MatchType uint8

const (
	// ContainMatch is for contain match
	ContainMatch MatchType = iota
	// ExactMatch is for exact match
	ExactMatch
	// UnwantedContainMatch checks that output does not contain text
	UnwantedContainMatch
	// UnwantedExactMatch checks that output does not exactly match text
	UnwantedExactMatch
	// RegexMatch is for regular expression match
	RegexMatch
)

func (m MatchType) String() string {
	switch m {
	case ContainMatch:
		return "ContainMatch"
	case ExactMatch:
		return "ExactMatch"
	case UnwantedContainMatch:
		return "UnwantedContainMatch"
	case UnwantedExactMatch:
		return "UnwantedExactMatch"
	case RegexMatch:
		return "RegexMatch"
	default:
		return "unknown match"
	}
}

// streamType defines a stream type
type streamType uint8

const (
	// outputStream is the command output stream
	outputStream streamType = iota
	// errorStream is the command error stream
	errorStream
)

func (r *ApptainerCmdResult) expectMatch(mt MatchType, stream streamType, pattern string) error {
	var output string
	var streamName string

	switch stream {
	case outputStream:
		output = string(r.Stdout)
		streamName = "output"
	case errorStream:
		output = string(r.Stderr)
		streamName = "error"
	}

	switch mt {
	case ContainMatch:
		if !strings.Contains(output, pattern) {
			return errors.Errorf(
				"Command %q:\nExpect %s stream contains:\n%s\nCommand %s stream:\n%s",
				r.FullCmd, streamName, pattern, streamName, output,
			)
		}
	case ExactMatch:
		// get rid of the trailing newline
		if strings.TrimSuffix(output, "\n") != pattern {
			return errors.Errorf(
				"Command %q:\nExpect %s stream exact match:\n%s\nCommand %s output:\n%s",
				r.FullCmd, streamName, pattern, streamName, output,
			)
		}
	case UnwantedContainMatch:
		if strings.Contains(output, pattern) {
			return errors.Errorf(
				"Command %q:\nExpect %s stream does not contain:\n%s\nCommand %s stream:\n%s",
				r.FullCmd, streamName, pattern, streamName, output,
			)
		}
	case UnwantedExactMatch:
		if strings.TrimSuffix(output, "\n") == pattern {
			return errors.Errorf(
				"Command %q:\nExpect %s stream not matching:\n%s\nCommand %s output:\n%s",
				r.FullCmd, streamName, pattern, streamName, output,
			)
		}
	case RegexMatch:
		matched, err := regexp.MatchString(pattern, output)
		if err != nil {
			return errors.Errorf(
				"compilation of regular expression %q failed: %s",
				pattern, err,
			)
		}
		if !matched {
			return errors.Errorf(
				"Command %q:\nExpect %s stream match regular expression:\n%s\nCommand %s output:\n%s",
				r.FullCmd, streamName, pattern, streamName, output,
			)
		}
	}

	return nil
}

// ExpectOutput tests if the command output stream match the
// pattern string based on the type of match.
func ExpectOutput(mt MatchType, pattern string) ApptainerCmdResultOp {
	return func(t *testing.T, r *ApptainerCmdResult) {
		t.Helper()

		err := r.expectMatch(mt, outputStream, pattern)
		err = errors.Wrapf(err, "matching %q of type %s in output stream", pattern, mt)
		if err != nil {
			t.Errorf("failed to match pattern: %+v", err)
		}
	}
}

// ExpectOutputf tests if the command output stream match the
// formatted string pattern based on the type of match.
func ExpectOutputf(mt MatchType, formatPattern string, a ...interface{}) ApptainerCmdResultOp {
	return func(t *testing.T, r *ApptainerCmdResult) {
		t.Helper()

		pattern := fmt.Sprintf(formatPattern, a...)
		err := r.expectMatch(mt, outputStream, pattern)
		err = errors.Wrapf(err, "matching %q of type %s in output stream", pattern, mt)
		if err != nil {
			t.Errorf("failed to match pattern: %+v", err)
		}
	}
}

// ExpectError tests if the command error stream match the
// pattern string based on the type of match.
func ExpectError(mt MatchType, pattern string) ApptainerCmdResultOp {
	return func(t *testing.T, r *ApptainerCmdResult) {
		t.Helper()

		err := r.expectMatch(mt, errorStream, pattern)
		err = errors.Wrapf(err, "matching %q of type %s in output stream", pattern, mt)
		if err != nil {
			t.Errorf("failed to match pattern: %+v", err)
		}
	}
}

// ExpectErrorf tests if the command error stream match the
// pattern string based on the type of match.
func ExpectErrorf(mt MatchType, formatPattern string, a ...interface{}) ApptainerCmdResultOp {
	return func(t *testing.T, r *ApptainerCmdResult) {
		t.Helper()

		pattern := fmt.Sprintf(formatPattern, a...)
		err := r.expectMatch(mt, errorStream, pattern)
		err = errors.Wrapf(err, "matching %q of type %s in output stream", pattern, mt)
		if err != nil {
			t.Errorf("failed to match pattern: %+v", err)
		}
	}
}

// GetStreams gets command stdout and stderr result.
func GetStreams(stdout *string, stderr *string) ApptainerCmdResultOp {
	return func(t *testing.T, r *ApptainerCmdResult) {
		t.Helper()

		*stdout = string(r.Stdout)
		*stderr = string(r.Stderr)
	}
}

// ApptainerConsoleOp is a function type passed to ConsoleRun
// to execute interactive commands.
type ApptainerConsoleOp func(*testing.T, *expect.Console)

// ConsoleExpectf reads from the console until the provided formatted string
// is read or an error occurs.
func ConsoleExpectf(format string, args ...interface{}) ApptainerConsoleOp {
	return func(t *testing.T, c *expect.Console) {
		t.Helper()

		if o, err := c.Expectf(format, args...); err != nil {
			err = errors.Wrap(err, "checking console output")
			expected := fmt.Sprintf(format, args...)
			t.Logf("\nConsole output: %s\nExpected output: %s", o, expected)
			t.Errorf("error while reading from the console: %+v", err)
		}
	}
}

// ConsoleExpect reads from the console until the provided string is read or
// an error occurs.
func ConsoleExpect(s string) ApptainerConsoleOp {
	return func(t *testing.T, c *expect.Console) {
		t.Helper()

		if o, err := c.ExpectString(s); err != nil {
			err = errors.Wrap(err, "checking console output")
			t.Logf("\nConsole output: %s\nExpected output: %s", o, s)
			t.Errorf("error while reading from the console: %+v", err)
		}
	}
}

// ConsoleSend writes a string to the console.
func ConsoleSend(s string) ApptainerConsoleOp {
	return func(t *testing.T, c *expect.Console) {
		t.Helper()

		if _, err := c.Send(s); err != nil {
			err = errors.Wrapf(err, "sending %q to console", s)
			t.Errorf("error while writing string to the console: %+v", err)
		}
	}
}

// ConsoleSendLine writes a string to the console with a trailing newline.
func ConsoleSendLine(s string) ApptainerConsoleOp {
	return func(t *testing.T, c *expect.Console) {
		t.Helper()

		if _, err := c.SendLine(s); err != nil {
			err = errors.Wrapf(err, "sending line %q to console", s)
			t.Errorf("error while writing string to the console: %+v", err)
		}
	}
}

// ApptainerCmdOp is a function type passed to RunCommand
// used to define the test execution context.
type ApptainerCmdOp func(*apptainerCmd)

// apptainerCmd defines an Apptainer command execution test.
type apptainerCmd struct {
	globalOptions []string
	cmd           []string
	args          []string
	envs          []string
	dir           string // Working directory to be used when executing the command
	subtestName   string
	stdin         io.Reader
	waitErr       error
	preFn         func(*testing.T)
	postFn        func(*testing.T)
	consoleFn     ApptainerCmdOp
	console       *expect.Console
	resultFn      ApptainerCmdOp
	result        *ApptainerCmdResult
	t             *testing.T
	profile       Profile
}

// AsSubtest requests the command to be run as a subtest
func AsSubtest(name string) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.subtestName = name
	}
}

// WithCommand sets the apptainer command to execute.
func WithCommand(command string) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.cmd = strings.Split(command, " ")
	}
}

// WithArgs sets the apptainer command arguments.
func WithArgs(args ...string) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		if len(args) > 0 {
			s.args = append(s.args, args...)
		}
	}
}

// WithEnv sets environment variables to use while running a
// apptainer command.
func WithEnv(envs []string) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		if len(envs) > 0 {
			s.envs = append(s.envs, envs...)
		}
	}
}

// WithRootlessEnv passes through XDG_RUNTIME_DIR and DBUS_SESSION_BUS_ADDRESS
// for rootless operations that need these e.g. systemd cgroups interaction.
func WithRootlessEnv() ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.envs = append(s.envs, "XDG_RUNTIME_DIR="+os.Getenv("XDG_RUNTIME_DIR"))
		s.envs = append(s.envs, "DBUS_SESSION_BUS_ADDRESS="+os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
	}
}

// WithDir sets the current working directory for the execution of a command.
func WithDir(dir string) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		if dir != "" {
			s.dir = dir
		}
	}
}

// WithProfile sets the Apptainer execution profile, this
// is a convenient way to automatically set requirements like
// privileges, arguments injection in order to execute
// Apptainer command with the corresponding profile.
// RootProfile, RootUserNamespaceProfile will set privileges which
// means that PreRun and PostRun are executed with privileges.
func WithProfile(profile Profile) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.profile = profile
	}
}

// WithStdin sets a reader to use as input data to pass
// to the apptainer command.
func WithStdin(r io.Reader) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.stdin = r
	}
}

// WithGlobalOptions sets global apptainer option (eg: --debug, --silent).
func WithGlobalOptions(options ...string) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		if len(options) > 0 {
			s.globalOptions = append(s.globalOptions, options...)
		}
	}
}

// ConsoleRun sets console operations to interact with the
// running command.
func ConsoleRun(consoleOps ...ApptainerConsoleOp) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		if s.consoleFn == nil {
			s.consoleFn = ConsoleRun(consoleOps...)
			return
		}
		for _, op := range consoleOps {
			op(s.t, s.console)
		}
	}
}

// PreRun sets a function to execute before running the
// apptainer command, this function is executed with
// privileges if the profile is either RootProfile or
// RootUserNamespaceProfile.
func PreRun(fn func(*testing.T)) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.preFn = fn
	}
}

// PostRun sets a function to execute when the apptainer
// command execution finished, this function is executed with
// privileges if the profile is either RootProfile or
// RootUserNamespaceProfile. PostRun is executed in all cases
// even when the command execution failed, it's the responsibility
// of the caller to check if the test failed with t.Failed().
func PostRun(fn func(*testing.T)) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		s.postFn = fn
	}
}

// ExpectExit is called once the command completed and before
// PostRun function in order to check the exit code returned. This
// function is always required by RunCommand and can call additional
// test functions processing the command result like ExpectOutput,
// ExpectError.
func ExpectExit(code int, resultOps ...ApptainerCmdResultOp) ApptainerCmdOp {
	return func(s *apptainerCmd) {
		if s.resultFn == nil {
			s.resultFn = ExpectExit(code, resultOps...)
			return
		}

		r := s.result
		t := s.t

		t.Helper()

		if t.Failed() {
			return
		}

		cause := errors.Cause(s.waitErr)
		switch x := cause.(type) {
		case *exec.ExitError:
			if status, ok := x.Sys().(syscall.WaitStatus); ok {
				exitCode := status.ExitStatus()
				if status.Signaled() {
					s := status.Signal()
					exitCode = 128 + int(s)
				}
				if code != exitCode {
					t.Logf("\n%q output:\n%s%s\n", r.FullCmd, string(r.Stderr), string(r.Stdout))
					t.Errorf("got %d as exit code and was expecting %d: %+v", exitCode, code, s.waitErr)
					return
				}
			}
		default:
			if s.waitErr != nil {
				t.Errorf("command execution of %q failed: %+v", r.FullCmd, s.waitErr)
				return
			}
		}

		if code == 0 && s.waitErr != nil {
			t.Logf("\n%q output:\n%s%s\n", r.FullCmd, string(r.Stderr), string(r.Stdout))
			t.Errorf("unexpected failure while executing %q", r.FullCmd)
			return
		} else if code != 0 && s.waitErr == nil {
			t.Logf("\n%q output:\n%s%s\n", r.FullCmd, string(r.Stderr), string(r.Stdout))
			t.Errorf("unexpected success while executing %q", r.FullCmd)
			return
		}

		for _, op := range resultOps {
			if op != nil {
				op(t, r)
			}
		}
	}
}

// RunApptainer executes an Apptainer command within a test execution
// context.
//
// cmdPath specifies the path to the apptainer binary and cmdOps
// provides a list of operations to be executed before or after running
// the command.
//
//nolint:maintidx
func (env TestEnv) RunApptainer(t *testing.T, cmdOps ...ApptainerCmdOp) {
	t.Helper()

	cmdPath := env.CmdPath
	s := new(apptainerCmd)

	for _, op := range cmdOps {
		op(s)
	}
	if s.resultFn == nil {
		t.Errorf("ExpectExit is missing in cmdOps argument")
		return
	}

	// a profile is required
	if s.profile.name == "" {
		i := 0
		availableProfiles := make([]string, len(Profiles))
		for profile := range Profiles {
			availableProfiles[i] = profile
			i++
		}
		profiles := strings.Join(availableProfiles, ", ")
		t.Errorf("you must specify a profile, available profiles are %s", profiles)
		return
	}

	// the profile returns if it requires privileges or not
	privileged := s.profile.privileged

	fn := func(t *testing.T) {
		t.Helper()

		s.result = new(ApptainerCmdResult)
		pargs := append(s.globalOptions, s.cmd...)
		pargs = append(pargs, s.profile.args(s.cmd)...)
		s.args = append(pargs, s.args...)
		s.result.FullCmd = fmt.Sprintf("%s %s", cmdPath, strings.Join(s.args, " "))

		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)

		// check if profile can run this test or skip it
		s.profile.Requirements(t)

		s.t = t

		cmd := exec.Command(cmdPath, s.args...)

		cmd.Env = s.envs
		if len(cmd.Env) == 0 {
			cmd.Env = os.Environ()
		}

		// By default, each E2E command shares a temporary image cache
		// directory. If a test is directly testing the cache, or depends on
		// specific ordered cache behavior then
		// TestEnv.PrivCacheDir/UnPrivCacheDir should be overridden to a
		// separate path in the test. The caller is then responsible for
		// creating and cleaning up the cache directory.
		cacheDir := env.UnprivCacheDir
		if privileged {
			cacheDir = env.PrivCacheDir
		}
		cacheDirEnv := fmt.Sprintf("%s=%s", cache.DirEnv, cacheDir)
		cmd.Env = append(cmd.Env, cacheDirEnv)

		// Each command gets by default a clean temporary PGP keyring.
		// If it is needed to share a keyring between tests, or to manually
		// set the directory to be used, one shall set the KeyringDir of the
		// test environment. Doing so will overwrite the default creation of
		// a keyring for the command to be executed/ In that context, it is
		// the developer's responsibility to ensure that the directory is
		// correctly deleted upon successful or unsuccessful completion of the
		// test.
		keysDir := env.KeyringDir

		if keysDir == "" {
			// cleanKeyring is a function that will delete the temporary
			// PGP keyring and fail the test if it cannot be deleted.
			keyringDir, cleanKeysDir := MakeKeysDir(t, env.TestDir)
			keysDir = keyringDir
			defer cleanKeysDir(t)
		}
		keysDirEnv := fmt.Sprintf("%s=%s", "APPTAINER_KEYSDIR", keysDir)
		cmd.Env = append(cmd.Env, keysDirEnv)

		// We check if we need to disable the cache
		if env.DisableCache {
			cmd.Env = append(cmd.Env, "APPTAINER_DISABLE_CACHE=1")
		}

		cmd.Dir = s.dir
		if cmd.Dir == "" {
			cmd.Dir = s.profile.defaultCwd
		}

		// when duplicated environment variables are found
		// only the last one in the slice is taken
		if cmd.Dir != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("PWD=%s", cmd.Dir))
		}

		// Set $HOME
		if env.HomeDir == "" {
			env.HomeDir = CurrentUser(t).Dir
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("HOME=%s", env.HomeDir))

		// propagate proxy environment variables
		for _, env := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"} {
			val := os.Getenv(env)
			if val != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env, val))
			}
		}

		cmd.Stdin = s.stdin
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if s.preFn != nil {
			s.preFn(t)
			// if PreRun call t.Error(f) or t.Skip(f), don't
			// execute the command and return
			if t.Failed() || t.Skipped() {
				return
			}
		}
		if s.consoleFn != nil {
			var err error

			// NewTestConsole is prone to race, use NewConsole for now
			s.console, err = expect.NewConsole(
				expect.WithStdout(cmd.Stdout, cmd.Stderr),
				expect.WithDefaultTimeout(10*time.Second),
			)
			err = errors.Wrap(err, "creating expect console")
			if err != nil {
				t.Errorf("console initialization failed: %+v", err)
				return
			}
			defer s.console.Close()

			cmd.Stdin = s.console.Tty()
			cmd.Stdout = s.console.Tty()
			cmd.Stderr = s.console.Tty()
			cmd.ExtraFiles = []*os.File{s.console.Tty()}

			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setctty: true,
				Setsid:  true,
				Ctty:    3,
			}
		}
		if s.postFn != nil {
			defer s.postFn(t)
		}

		t.Logf("Running command %q", s.result.FullCmd)

		if err := cmd.Start(); err != nil {
			err = errors.Wrapf(err, "running command %q", s.result.FullCmd)
			t.Errorf("command execution of %q failed: %+v", s.result.FullCmd, err)
			return
		}

		if s.consoleFn != nil {
			s.consoleFn(s)

			s.waitErr = errors.Wrapf(cmd.Wait(), "waiting for command %q", s.result.FullCmd)
			// close I/O on our side
			if err := s.console.Tty().Close(); err != nil {
				t.Errorf("error while closing console: %s", err)
				return
			}
			_, err := s.console.Expect(expect.EOF, expect.PTSClosed, expect.WithTimeout(1*time.Second))
			// we've set a shorter timeout of 1 second, we simply ignore it because
			// it means that the command didn't close I/O streams and keep running
			// in background
			if err != nil && !os.IsTimeout(err) {
				t.Errorf("error while waiting console EOF: %s", err)
				return
			}
		} else {
			s.waitErr = errors.Wrapf(cmd.Wait(), "waiting for command %q", s.result.FullCmd)
		}

		s.result.Stdout = stdout.Bytes()
		s.result.Stderr = stderr.Bytes()
		s.resultFn(s)
	}

	if privileged {
		fn = Privileged(fn)
	}

	if s.subtestName != "" {
		t.Run(s.subtestName, fn)
	} else {
		var wg sync.WaitGroup

		// if this is not a subtest, we will execute the above
		// function in a separated go routine like t.Run would do
		// in order to not mess up with privileges if a subsequent
		// RunApptainer is executed without being a sub-test in
		// PostRun
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(t)
		}()
		wg.Wait()
	}
}
