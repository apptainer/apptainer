# End-to-End Testing

This package contains the end-to-end tests for `apptainer`.

## Basic e2e infrastructure

### Initialization and the e2e.TestEnv struct

The `e2e.TestEnv` struct is created and initialized in the e2e.Run() function,
defined in the e2e/suite.go file. This function initializes many fields of the
struct and carries out initialization procedures that are necessary for the
entire e2e suite to work. Here are a few examples:

- Creating a temporary test directory (intended to serve as the parent dir for
  any more specific temporary subdirs that may be needed in the course of
  specific tests), and setting the `TestDir` field of the struct to point to
  that directory.
- Setting up "fake" home directories for the current user and for root, so that
  actions that affect the home dir (caches, tokens, config changes, etc.) will
  not affect the files in the users' real home dirs.
  - This is achieved by creating temporary homedirs for the user and for root,
    and bind-mounting them over the real ones (`$HOME` and `/root`,
    respectively).
  - Because the e2e suite is run inside a dedicated mount namespace, this
    bind-mount does not affect the "outside world."
  - The actual function that is called to set up these fake homedirs is
    SetupHomeDirectories(), defined in e2e/internal/e2e/home.go
- Blank/default versions of the following are set up & placed in the
  aforementioned temporary `TestDir`:
  - `apptainer.conf`
  - `remote.yaml`
  - plugin dir
  - ECL configuration
  - Global keyring
- If the E2E_DOCKER_USERNAME and E2E_DOCKER_PASSWORD environment variables are
  set, they will be used to generate `docker-config.json` files, which will be
  placed inside the `.apptainer` subdir of the "fake" user homedir and the
  fake `/root` (see above).
  - By supplying login credentials to DockerHub in this fashion, one can run e2e
    tests without hitting the rate-limits that apply when accessing DockerHub
    anonymously.

#### The local Docker/OCI registry

Next, a local Docker/OCI registry is spun up for testing purposes. The host &
address for this local testing registry is stored in `testenv.TestRegistry`
(note that the string stored here does *not* contain the `docker://` transport
prefix).

A few images are immediately pushed to this testing registry; others are only
generated on demand, using the `testenv.EnsureXYZ()` functions, discussed below.

The images immediately pushed to the testing registry (in what follows,
`<registryURI>` should be understood as shorthand for
`"docker://"+testenv.TestRegistry`):

- `<registryURI>/my-busybox:latest`:
  - Created by copying `docker://busybox:latest` at runtime.
- `<registryURI>/private/e2eprivrepo/my-busybox:latest`:
  - Another copy of `docker://busybox:latest` created at runtime, but pushed into
  a private location in the testing repo that requires authentication.
    - To push the private image, e2e.Run() makes use of the
      PrivateRepoLogin()/PrivateRepoLogout() functions defined in
      e2e/internal/e2e/private_repo.go. See the comments on those functions for
      more information.

The following URIs are then stored in `testenv` fields for convenience:

- `testenv.TestRegistryImage` = `<registryURI>/my-busybox:latest`
- `testenv.TestRegistryPrivURI` = `<registryURI>`
  - At present, this is simply identical to `"docker://"+testenv.TestRegistry`.
    But the test suite is written so that this *could* point at a different
    registry.
- `testenv.TestRegistryPrivPath` = `testenv.TestRegistryPrivURI+"/private/e2eprivrepo"`
- `testenv.TestRegistryPrivImage` = `testenv.TestRegistryPrivPath+"docker://%s/my-busybox:latest"`

#### The EnsureXYZ() functions

Aside from the images copied as part of setting up [the local Docker/OCI
registry](#the-local-dockeroci-registry), the e2e suite makes a series of other
images available on-demand: these images are copied or built only when a
particular EnsureXYZ() function is called.

Below is a description of each of the EnsureXYZ() functions, and the image they
create. These functions are defined in e2e/internal/e2e/image.go, and they use
mutexes to ensure they are concurrency-safe, and that the initialization they
perform is only ever done once in the course of an entire e2e suite run.

- EnsureImage():
  - Builds the "main" test image, whose definition is located in
    e2e/testdata/Apptainer.
  - The image is saved to a file named "test.sif" inside `testenv.TestDir`.
  - The path to this image file is saved in `testenv.ImagePath`.
- EnsureORASImage():
  - Pushes the SIF in `testenv.ImagePath` (see above, under EnsureImage()) to
    the local testing registry via the ORAS protocol.
  - The image is pushed to `<registryURI>/oras_test_sif:latest`, and this URI is
    saved in `testenv.OrasTestImage`.
- EnsureSingularityImage():
  - Builds a SIF container that includes an installation of a legacy version
    of Singularity (for ensuring compatibility), whose definition is located
    in e2e/testdata/Singularity_legacy.def.
  - The path to this image file is saved in `testenv.SingularityImagePath`.
- EnsureDebianImage():
  - Builds a SIF container based on an Ubuntu operating system version
    that has a libc compatible with the libc of the host running the e2e
    tests.
  - The path to this image file is saved in `testenv.DebianImagePath`.

**Any test whose correct operation depends on the existence of one of the images
listed here should begin by calling the corresponding EnsureXYZ() function.**

Remember that the actual initialization carried out by these functions will only
ever happen once, and so the performance cost of an EnsureXYZ() call to
initialize an image that has already been initialized is negligible.
  
## Profiles

The e2e suite defines a set of **profiles** representing different ways that
Apptainer might be run. For example, running Apptainer as root; running
Apptainer as a regular user with the `--fakeroot` flag; and so forth.

Profiles control the following aspects of Apptainer execution:

- Whether Apptainer is run as root or as a regular user.
- The default CWD (current working directory) in which to run Apptainer.
  *(optional)*
- A set of options (e.g. `--fakeroot`) to pass to Apptainer CLI commands.
- A gating function that will only let tests in this profile run under
  particular conditions. Note that this function does not return a boolean
  value; instead, it receives the `*testing.T` object corresponding to the
  current Go test, and calls `t.Skip()` if the conditions aren't met.
- The UID on the host.
- The UID in-container.

The e2e suite defines, in e2e/internal/e2e/profile.go, the following profiles:

- **UserProfile**: a regular user
- **RootProfile**: root
- **FakerootProfile**: fakeroot
- **UserNamespaceProfile**: a regular user and a user namespace
- **RootUserNamespaceProfile**: root and a user namespace

To see the particular values that each of these profiles sets, please consult
e2e/internal/e2e/profile.go.

Variables with the bolded names above are defined globally in e2e/internal/e2e,
so that to access RootProfile from outside the e2e/internal/e2e package, for
example, one would typically write `e2e.RootProfile`.

Individual profiles define a set of public methods. E.g. `p.Privileged()` will
return a `bool` value indicating whether `p` is a root profile. See
e2e/internal/e2e/profile.go for the full set of public methods.

## Invoking the Apptainer CLI

An e2e test typically proceeds by invoking the Apptainer CLI one or more
times. In order to use the Apptainer CLI from within the e2e suite in a way
that respects [profiles](#profiles) and interacts with Go's `*testing.T`
structure correctly, the e2e suite defines the testenv.RunApptainer()
function:

```go
func (c ctx) echoEnv(t *testing.T) {
    e2e.EnsureImage(t, c.env)

    env := []string{`FOO=BAR`}
    c.env.RunApptainer(
        t,
        e2e.WithProfile(e2e.UserProfile),
        e2e.WithCommand("exec"),
        e2e.WithEnv(env),
        e2e.WithArgs("/bin/sh", "-c" "echo $FOO"),
        e2e.ExpectExit(
            0,
            e2e.ExpectOutput(e2e.ExactMatch, "BAR"),
        ),
    )
}
```

### Functional options to testenv.RunApptainer()

The first argument to testenv.RunApptainer() is Go's `*testing.T` object for
the current test. This is followed by one or more [functional
options](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis).
The e2e suite defines a whole host of functional options for
testenv.RunApptainer(), and we will highlight only some of them here; the
reader should consult e2e/internal/e2e/apptainercmd.go for the full set of
options.

- **e2e.AsSubtest(string)**:
  - Executes the apptainer command in a separate named *subtest* of the
    current test (passed in the first argument of testenv.RunApptainer()).
- **e2e.WithProfile(e2e.Profile)**:
  - The [profile](#profiles) for this run of the CLI.
- **e2e.WithCommand(string)**:
  - The CLI command to execute. (E.g. to run `apptainer help`, one would pass
    `e2e.WithCommand("help")` as one of the functional options to
    testenv.RunApptainer()).
- **e2e.WithArgs(...string)**:
  - Additional arguments to pass to the CLI command defined in
    e2e.WithCommand(), above.
    - **Important:** Not all arguments to the CLI belong here - in particular,
      those that are specified by [profiles](#profiles), such as
      `--fakeroot`, should be provided by choosing the correct profile in
      e2e.WithProfile(), above.
- **e2e.WithEnv([]string)**:
  - Environment variables to set for this run of the CLI.
- **e2e.WithDir(string)**:
  - The current working directory in which to run the CLI.
- **e2e.PreRun(func(\*testing.T))** and **e2e.PostRun(func(\*testing.T))**:
  - Code to run before and after the CLI run itself. Note that the function
    passed as an argument to PreRun()/PostRun() receives the Go `*testing.T`
    object as an argument, and returns no values. Therefore, the function is
    expected to use methods like t.Skip(), t.Error()/t.Errorf(),
    t.Fatal()/t.Fatalf(), etc., as appropriate.
- **e2e.ExpectExit()**:
  - Discussed separately, [below](#the-e2eexpectexit-option).

### The e2e.ExpectExit() option

The functional argument e2e.ExpectExit() is more complex than
testenv.RunApptainer()'s other functional arguments, and deserves to be
discussed in slightly more detail.

The purpose of this argument is to define what is expected of this CLI run, such
that the test will be considered to have failed (specifically, the Fail() method
of the `*testing.T` object will be called) if the expected conditions are not
met.

Here, once again, is the `e2e.ExpectExit()` functional argument from the earlier
example:

```go
        e2e.ExpectExit(
            0,
            e2e.ExpectOutput(e2e.ExactMatch, "BAR"),
        ),
```

The first argument is the Unix [exit
code](https://www.baeldung.com/linux/status-codes) the test expects the CLI to
return. (Zero, as in this example, means that the CLI run has terminated
successfully; though that does not yet mean it did what we expected; keep
reading!)

The rest of the arguments to e2e.ExpectExit() are zero or more [functional
options](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
again. The most common functional options to e2e.ExpectExit() are
**e2e.ExpectOutput()** and **e2e.ExpectError()**, for examining stdout and
stderr output, respectively. (Note that much like fmt.Print() has a fmt.Printf()
counterpart, so too do e2e.ExpectOutput() and e2e.ExpectError() have
e2e.ExpectOutputf() and e2e.ExpectErrorf() counterparts that allow for standard
Go string formatting directives. See e2e/internal/e2e/apptainercmd.go for
details.)

*Note: Since the APIs of e2e.ExpectOutput() and e2e.ExpectError() are identical,
we will discuss e2e.ExpectOutput() from here on out, but the same applies to
e2e.ExpectError() as well.*

e2e.ExpectOutput() takes two arguments. The first is the **match type**, and the
second is a string. (Or, in the e2e.Expect{Output,Error}f variants, a string
followed by a set of arguments corresponding to the format directives in the
string.) The following match types are defined in
e2e/internal/e2e/apptainercmd.go:

- **ContainMatch**:
  - For the test to pass, the output must contain the string in the second
    argument.
- **ExactMatch**:
  - For the test to pass, the output must be equal to string in the second
    argument. (In particular, it cannot contain anything before or after this
    string, apart from a final newline.)
- **UnwantedContainMatch**:
  - For the test to pass, the output must *not* contain the string in the second
    argument.
- **UnwantedExactMatch**:
  - For the test to pass, the output must *not* be equal to string in the second
    argument.
- **RegexMatch**:
  - For the test to pass, it must match the [regular
    expression](https://github.com/google/re2/wiki/Syntax) specified in the
    second argument.

Thus, in the code snippet above, we see that for the test to pass, the CLI must
return the exit code 0 (indicating a successful run), and the output from
stdout (disregarding stderr) must be exactly the string `BAR` - no more, and no
less, up to a terminating newline.

While this particular example tests what is expected to be a successful run,
these same options also let you test that Apptainer errors out correctly under
the circumstances where you want it to do so. You would typically do that by
combining an appropriate non-zero exit status as the first argument to
e2e.ExpectExit(), with an additional e2e.ExpectError() (or e2e.ExpectErrorf())
functional option to verify that the stderr output of the CLI run is what you
want it to be.

## Best practices in writing e2e tests

### Inline struct arrays for subtests

While the code snippets given so far demonstrate a single execution of
testenv.RunApptainer(), it is quite common for a single test to run
testenv.RunApptainer() multiple times, each time modifying something about the
run conditions.

To enhance the readability of the e2e sources, tests of this sort should be
written using the [table driven test
pattern](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests). As an
example, here is the exitSignals() test from e2e/actions/actions.go:

```go
func (c actionTests) exitSignals(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name string
		args []string
		exit int
	}{
		{
			name: "Exit0",
			args: []string{c.env.ImagePath, "/bin/sh", "-c", "exit 0"},
			exit: 0,
		},
		{
			name: "Exit1",
			args: []string{c.env.ImagePath, "/bin/sh", "-c", "exit 1"},
			exit: 1,
		},
		{
			name: "Exit134",
			args: []string{c.env.ImagePath, "/bin/sh", "-c", "exit 134"},
			exit: 134,
		},
		{
			name: "SignalKill",
			args: []string{c.env.ImagePath, "/bin/sh", "-c", "kill -KILL $$"},
			exit: 137,
		},
		{
			name: "SignalAbort",
			args: []string{c.env.ImagePath, "/bin/sh", "-c", "kill -ABRT $$"},
			exit: 134,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit),
		)
	}
}
```

Instead of writing a series of calls to RunApptainer() with slightly different
arguments each time, we define a new inline struct type containing only the
properties we want to vary in each subtest (in this case, only the additional
arguments to `exec` and the expected exit code, alongside the name for the
subtest). We place the array of these structs into a local variable (`tests`),
and iterate over this array to create the actual CLI calls we are interested in.

This approach cleanly separates the varying aspects of each subtest (contained
in the struct array) from those that remain constant (coded in the body of the
for-loop).

Note that the struct type defined here includes a field `name`, which we use to
execute each CLI run as its own separate subtest (by passing
`e2e.AsSubtest(tt.name)` as one of the functional options to
RunApptainer()).
This is important, because otherwise Go would end up affixing a running counter
to the main test name, which would make test logs a lot less informative as far
as where test failures have/haven't occurred. Subtest names should strike a
balance between clearly representing what the subtest does, on the one hand, and
brevity, on the other. Brevity is important here because
full test names can get rather long once the
names of nested subsets are all spelled out.

Anything that can be passed in a functional argument to RunApptainer() can be
part of the struct type we define. The following is the actionCompat() test from
e2e/actions/actions.go:

```go
func (c actionTests) actionCompat(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	type test struct {
		name     string
		args     []string
		exitCode int
		expect   e2e.ApptainerCmdResultOp
	}

	tests := []test{
		{
			name:     "containall",
			args:     []string{"--compat", c.env.ImagePath, "sh", "-c", "ls -lah $HOME"},
			exitCode: 0,
			expect:   e2e.ExpectOutput(e2e.ContainMatch, "total 0"),
		},
		{
			name:     "writable-tmpfs",
			args:     []string{"--compat", c.env.ImagePath, "sh", "-c", "touch /test"},
			exitCode: 0,
		},
		{
			name:     "no-init",
			args:     []string{"--compat", c.env.ImagePath, "sh", "-c", "ps"},
			exitCode: 0,
			expect:   e2e.ExpectOutput(e2e.UnwantedContainMatch, "sinit"),
		},
		{
			name:     "no-umask",
			args:     []string{"--compat", c.env.ImagePath, "sh", "-c", "umask"},
			exitCode: 0,
			expect:   e2e.ExpectOutput(e2e.ContainMatch, "0022"),
		},
	}

	oldUmask := syscall.Umask(0)
	defer syscall.Umask(oldUmask)

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(
				tt.exitCode,
				tt.expect,
			),
		)
	}
}
```

Here, we pass different [e2e.ExpectOutput()](#the-e2eexpectexit-option) options
for each of the different subtests (even passing none at all, for the
"writable-tmpfs" subtest).

Or consider the testCLICallbacks() test in e2e/plugin/plugin.go:

```go
func (c ctx) testCLICallbacks(t *testing.T) {
	pluginDir := "./plugin/testdata/cli"
	pluginName := "github.com/sylabs/apptainer/e2e-cli-plugin"

	// plugin sif file
	sifFile := filepath.Join(c.env.TestDir, "plugin.sif")
	defer os.Remove(sifFile)

	tests := []struct {
		name       string
		profile    e2e.Profile
		command    string
		args       []string
		expectExit int
	}{
		{
			name:       "Compile",
			profile:    e2e.UserProfile,
			command:    "plugin compile",
			args:       []string{"--out", sifFile, pluginDir},
			expectExit: 0,
		},
		{
			name:       "Install",
			profile:    e2e.RootProfile,
			command:    "plugin install",
			args:       []string{sifFile},
			expectExit: 0,
		},
		{
			name:       "CLICallback",
			profile:    e2e.UserProfile,
			command:    "exit",
			args:       []string{"42"},
			expectExit: 42,
		},
		{
			name:       "ApptainerConfigCallback",
			profile:    e2e.UserProfile,
			command:    "shell",
			args:       []string{c.env.TestDir},
			expectExit: 43,
		},
		{
			name:       "Uninstall",
			profile:    e2e.RootProfile,
			command:    "plugin uninstall",
			args:       []string{pluginName},
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit),
		)
	}
}
```

Here, we see that both the *profile* and the *command* to be run vary from
subtest to subtest, so they have been included in the struct type that the
function defines.

### Iterating over profiles

Often we are interested in running the same test in different profiles. We could
do this by using multiple entries in a table driven test, varying the `profile`
field each time. But this is not the tidiest way to achieve this goal. That's
because we would be using a data structure intended to capture *everything that
varies from subtest to subtest* when, in reality, we're not varying anything
except the profile.

A better way to achieve this is to embed the call to RunApptainer() inside a
for-loop iterating over the set of profiles we want to test. For example:

```go
func (c actionTests) actionTmpSandboxFlag(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	profiles := []e2e.Profile{
    e2e.UserProfile, 
    e2e.RootProfile, 
    e2e.FakerootProfile, 
    e2e.UserNamespaceProfile,
  }

	for _, p := range profiles {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(p.String()),
			e2e.WithProfile(p),
			e2e.WithCommand("exec"),
			e2e.WithArgs("--sif-fuse=false", "--no-tmp-sandbox", "-u", c.env.ImagePath, "/bin/true"),
			e2e.ExpectExit(255),
		)
	}
}
```

It is common to pair this pattern with the [table driven test
pattern](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests) discussed
[above](#inline-struct-arrays-for-subtests), which can be easily done as
follows:

```go
func (c *ctx) testInstanceAuthFile(t *testing.T) {
	e2e.EnsureORASImage(t, c.env)
	instanceName := "actionAuthTesterInstance"
	localAuthFileName := "./my_local_authfile"
	authFileArgs := []string{"--authfile", localAuthFileName}

  <...>

	tests := []struct {
		name          string
		subCmd        string
		args          []string
		whileLoggedIn bool
		expectExit    int
	}{
		{
			name:          "start before auth",
			subCmd:        "start",
			args:          append(authFileArgs, "--disable-cache", "--no-https", c.env.TestRegistryPrivImage, instanceName),
			whileLoggedIn: false,
			expectExit:    255,
		},
		{
			name:          "start",
			subCmd:        "start",
			args:          append(authFileArgs, "--disable-cache", "--no-https", c.env.TestRegistryPrivImage, instanceName),
			whileLoggedIn: true,
			expectExit:    0,
		},
		{
			name:          "stop",
			subCmd:        "stop",
			args:          []string{instanceName},
			whileLoggedIn: true,
			expectExit:    0,
		},
		{
			name:          "start noauth",
			subCmd:        "start",
			args:          append(authFileArgs, "--disable-cache", "--no-https", c.env.TestRegistryPrivImage, instanceName),
			whileLoggedIn: false,
			expectExit:    255,
		},
	}

	profiles := []e2e.Profile{
		e2e.UserProfile,
		e2e.RootProfile,
	}

	for _, p := range profiles {
		t.Run(p.String(), func(t *testing.T) {
			for _, tt := range tests {
				if tt.whileLoggedIn {
					e2e.PrivateRepoLogin(t, c.env, p, localAuthFileName)
				} else {
					e2e.PrivateRepoLogout(t, c.env, p, localAuthFileName)
				}
				c.env.RunApptainer(
					t,
					e2e.AsSubtest(tt.name),
					e2e.WithProfile(p),
					e2e.WithCommand("instance "+tt.subCmd),
					e2e.WithArgs(tt.args...),
					e2e.ExpectExit(tt.expectExit),
				)
			}
		})
	}
}
```

Notice that we want to avoid running the same subtest in different profiles with
the same test name (in which case, Go would just affix a running counter to the
test names, which would not make for very readable output). For this reason, we
run the batch of tests in each profile as a separate subtest, using the
`t.Run(<subtest_name>, func(t *testing.T) {<...>})` method of Go's testing object.
We use the profile's `.String()` method to retrieve the profile's name, and use
that as the name of the subtest.

Overall, then, we end up with two levels of subtest nesting here: one level for
the profile, and another for the subtest names as defined in the struct array.
Here's an example of what the test output log looks like in this case:

```plain
=== RUN   TestE2E/SEQ/INSTANCE/auth
=== RUN   TestE2E/SEQ/INSTANCE/auth/User
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry logout --authfile ./my_local_authfile docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/User/start_before_auth
    instance.go:299: Running command "/usr/local/bin/apptainer instance start --authfile ./my_local_authfile --disable-cache --no-https docker://localhost:41151/private/e2eprivrepo/my-alpine:latest actionAuthTesterInstance"
=== NAME  TestE2E/SEQ/INSTANCE/auth/User
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry login --authfile ./my_local_authfile -u e2e -p e2e docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/User/start
    instance.go:299: Running command "/usr/local/bin/apptainer instance start --authfile ./my_local_authfile --disable-cache --no-https docker://localhost:41151/private/e2eprivrepo/my-alpine:latest actionAuthTesterInstance"
=== NAME  TestE2E/SEQ/INSTANCE/auth/User
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry login --authfile ./my_local_authfile -u e2e -p e2e docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/User/stop
    instance.go:299: Running command "/usr/local/bin/apptainer instance stop actionAuthTesterInstance"
=== NAME  TestE2E/SEQ/INSTANCE/auth/User
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry logout --authfile ./my_local_authfile docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/User/start_noauth
    instance.go:299: Running command "/usr/local/bin/apptainer instance start --authfile ./my_local_authfile --disable-cache --no-https docker://localhost:41151/private/e2eprivrepo/my-alpine:latest actionAuthTesterInstance"
=== RUN   TestE2E/SEQ/INSTANCE/auth/Root
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry logout --authfile ./my_local_authfile docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/Root/start_before_auth
    instance.go:299: Running command "/usr/local/bin/apptainer instance start --authfile ./my_local_authfile --disable-cache --no-https docker://localhost:41151/private/e2eprivrepo/my-alpine:latest actionAuthTesterInstance"
=== NAME  TestE2E/SEQ/INSTANCE/auth/Root
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry login --authfile ./my_local_authfile -u e2e -p e2e docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/Root/start
    instance.go:299: Running command "/usr/local/bin/apptainer instance start --authfile ./my_local_authfile --disable-cache --no-https docker://localhost:41151/private/e2eprivrepo/my-alpine:latest actionAuthTesterInstance"
=== NAME  TestE2E/SEQ/INSTANCE/auth/Root
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry login --authfile ./my_local_authfile -u e2e -p e2e docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/Root/stop
    instance.go:299: Running command "/usr/local/bin/apptainer instance stop actionAuthTesterInstance"
=== NAME  TestE2E/SEQ/INSTANCE/auth/Root
    apptainercmd.go:698: Running command "/usr/local/bin/apptainer registry logout --authfile ./my_local_authfile docker://localhost:41151"
=== RUN   TestE2E/SEQ/INSTANCE/auth/Root/start_noauth
    instance.go:299: Running command "/usr/local/bin/apptainer instance start --authfile ./my_local_authfile --disable-cache --no-https docker://localhost:41151/private/e2eprivrepo/my-alpine:latest actionAuthTesterInstance"
```

### Temporary dirs & files, and cleanup

Tests should be written so that the state of the filesystem after they run is
the same as it was before. To this end, it will typically be necessary to create
temporary files or even temporary directories.

As noted [above](#initialization-and-the-e2etestenv-struct), the initialization
code that runs at the beginning of the e2e suite creates a temporary directory
and stores its path in `testenv.TestDir`. Note however that this is **a single
directory for the entirety of this e2e run**, no matter how many individual
tests & subtests are run as part of it.

Therefore, an individual test or subtest should take active steps to avoid name
clashes for the temporary files & directories it creates. The best strategy for
this is as follows:

- **Location**: Temporary files & directories should be created under
  `testenv.TestDir`.
  - That way, if the same test was executed as part of a different run of the
    e2e suite, the files would be in different places (because `testenv.TestDir`
    differs per-run).
- **Naming**: Temporary files & directories should have a name that is unique to
  the test/subtest being run.
  - That way, temporary files & directories created by different tests/subtests
    in a single e2e run won't clash with one another.

The functions in Go's standard library for creating temporary files and for
creating temporary directories support customizing both the location and the
name of the file/dir, and so both these goals can be accomplished:

- **Files:**
  - The function `os.CreateTemp(dir, pattern string) (*File, error)` in the `os`
    package of the standard Go library accepts both a parent directory in which
    to create the file (`dir`) and a pattern for the filename to include
    (`pattern`). Typically, the pattern is used as a prefix, but other behaviors
    are possible. See the full documentation for this function
    [here](https://pkg.go.dev/os#CreateTemp).
  - The function `e2e.WriteTempFile(dir, pattern, content string) (string,
    error)` defined in e2e/internal/e2e/fileutil.go behaves similarly - indeed,
    it calls os.CreateTemp() with the `dir` and `pattern` arguments it is given.
    - It differs from os.CreateTemp() in that it opens the temporary file it
      created, writes the `content` to it, closes it, and returns the path to
      the temporary file as the first return value.
- **Directories:**
  - The function `os.MkdirTemp(dir, pattern string) (string, error)` in the `os`
    package of the standard Go library accepts both a parent directory in which
    to create the temporary subdir (`dir`) and a pattern for the dirname to
    include (`pattern`). Typically, the pattern is used as a prefix, but other
    behaviors are possible. See the full documentation for this function
    [here](https://pkg.go.dev/os#MkdirTemp).
  - The function `e2e.MakeTempDir(t *testing.T, baseDir string, prefix string,
    context string) (string, func(t *testing.T))` defined in
    e2e/internal/e2e/fileutil.go behaves similarly - indeed, it calls
    fs.MakeTmpDir() (defined in internal/pkg/util/fs/helper.go) with the `dir`
    and `pattern` arguments it is given, and fs.MakeTmpDir() in turn calls
    os.MkdirTemp() with these arguments.
    - It differs from os.MkdirTemp() in that it doesn't return an error value
      (any errors that arise will be issued as `t.Fatal(<...>)` errors to the
      `*testing.T` object passed as the first argument), and it returns,
      alongside the path to the created directory, a function that when called
      will remove the directory in question.
    - The latter is very useful for cleanup purposes, a topic we turn to
      presently.

Even if all files & directories are created in temporary locations as just
specified, tests should still clean up after themselves, removing any files and
directories they create. This can be done using `defer` statements, but the
preferred practice is to use the `t.Cleanup(f func())` method of Go's
`*testing.T` object.

There are several advantages to this approach. First, it allows for *conditional
cleanup*: it is often desirable, whether it be for debugging the e2e test itself
or for debugging an issue that these tests have revealed in Apptainer, to
retain the temporary files of a failed test. We can therefore make the cleanup
of a test conditional on that test having passed. Here is a typical example, in
this case using the second return value of e2e.MakeTempDir() discussed above to
perform the cleanup, taken from the actionOciOverlayTeardown() test in
e2e/actions/oci.go:

```go
	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "persistent-overlay-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})
```

In this example, the contents of the temporary directory (whose name will begin
with "persistent-overlay-", and whose full name can be read from the test
output) will be preserved in cases where the test fails.

The second advantage of t.Cleanup() over the use of `defer` statements concerns
the timing of their execution. While `defer` statements execute whenever the
current function returns, t.Cleanup() statements execute when the current named
test completes. This makes it possible to write a test that calls a helper
function, have that helper function create various temporary files/dirs *and*
set up their cleanup, and still use those files/dirs from the calling function,
because their cleanup will occur only when the entire named test finishes.

### Parallel (PAR) vs. non-parallel (SEQ) tests

Go's testing facility allows tests to be run in parallel, utilizing the compute
power of multicore systems. This is enabled by calling `t.Parallel()`, which the
suite.Run() function (in e2e/internal/testhelper/testhelper.go), called by the
main e2e.Run() function (in e2e/suite.go) does indeed call.

You should therefore assume, when writing an individual e2e test, that it *will*
run in parallel to other e2e tests.

In general, you should try your best to make your test safe to run in parallel.
For example, if your test involves building a new image file, don't just put the
file in the current directory; create a [dedicated temporary
directory](#temporary-dirs--files-and-cleanup) for this particular test under
`testenv.TestDir`, and build & use your image by specifying an absolute path to
your image file in that temporary subdir.

With that said, it is still the case that *some* tests cannot be run in parallel
to one another. Some examples include:

- Tests that require changing the current working directory.
  - Note that this does not include calls to
    [e2e.WithDir()](#functional-options-to-testenvrunapptainer) in
    testenv.RunApptainer(). These *are* safe to run in parallel, as they only
    affect the apptainer process that the test launches, not the process
    running the test code itself.
- Tests that require changing the OS
  [umask](https://en.wikipedia.org/wiki/Umask).
- Tests that affect files in the user's homedir (or in root's homedir, i.e.
  `/root`).
  - Even though the e2e suite sets up ["fake"
    homedirs](#initialization-and-the-e2etestenv-struct) for the current user
    and for root, those homedirs are still shared by the entire e2e run. And so,
    if two different tests were to manipulate files in the homedir at once, they
    could interfere with each other.
  - Examples where this concern arises include any test that would potentially
    change, or be sensitive to, the contents of files inside the user's
    `$HOME/.apptainer` directory, such as `remote.yaml`, `docker-config.json`,
    and others, as well as any test that changes the content of the system
    `apptainer.conf`.

To deal with such cases, e2e/internal/testhelper/testhelper.go defines a
function `testhelper.NoParallel(func(*testing.T)) func(*testing.T)`. This
function marks the test function it is given as an argument to run sequentially,
and not in parallel with any other tests.

It is for this reason that a typical test name in the e2e suite looks as follows:

```plain
<...>
TestE2E/PAR/BUILD/build_with_bind_mount
<...>
TestE2E/SEQ/DOCKER/cred_prio
<...>
```

`TestE2E` is the test name for the entire e2e suite; it is followed by `PAR`,
for the set of tests run in parallel, or by `SEQ`, for those tests that cannot
be run in parallel and are run sequentially.

For convenience, testhelper.NoParallel() returns its argument as its sole return
value. This makes it handy to use in the construction of `testhelper.Tests`
maps. Here, for example, is the E2ETests() function of the "REMOTE" tests group
(note that `testhelper.NoParallel` is assigned to the local variable `np` for
the sake of brevity, another best practice in writing E2ETests() functions):

```go
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"add":            c.remoteAdd,
		"list":           c.remoteList,
		"default or not": c.remoteDefaultOrNot,
		"remove":         c.remoteRemove,
		"status":         c.remoteStatus,
		"test help":      c.remoteTestHelp,
		"use":            c.remoteUse,
		"use exclusive":  np(c.remoteUseExclusive),
	}
}
```

As can be seen here, the c.remoteUseExclusive() test cannot be run in parallel,
but it can be marked for sequential running *and* added to the
`testhelper.Tests` map in one fell swoop, by making use of the return value of
testhelper.NoParallel().

## Useful utility functions

Some useful utility functions for e2e testing have already been discussed above,
including:

- [`e2e.WriteTempFile(dir, pattern, content string) (string,
  error)`](#temporary-dirs--files-and-cleanup)
- [`e2e.MakeTempDir(t *testing.T, baseDir string, prefix string, context string)
  (string, func(t *testing.T))`](#temporary-dirs--files-and-cleanup)

Here are some additional utility functions that are available, and which are
particularly useful for writing e2e tests:

- The `require` package
  ([internal/pkg/test/tool/require/require.go](https://github.com/apptainer/apptainer/blob/main/internal/pkg/test/tool/require/require.go))
  - While not strictly part of the e2e suite - it is available for use in
    unit-tests, as well - the `require` package defines a set of functions that
    allow you to gate a given test, so that it only runs if a particular
    requirement is met.
  - The functions in this package typically take, as their first argument, a `t
    *testing.T` object, and will call the t.Skip() function if the requirement
    is not satisfied (and will no-op if it is satisfied).
  - Some examples include:
    - `require.Filesystem(t *testing.T, fs string)`: only run the current test
      if the OS supports the filesystem named in `fs`
    - `require.Command(t *testing.T, command string)`: only run the current test
      if the executable `command` can be found on the path
      - Note that this function uses `bin.FindBin(command)` first, and only then
        falls back to `exec.LookPath(command)`, so that it emulates the same
        preferences (e.g. `squashfuse_ll` for `squashfuse`, if available) that
        Apptainer itself uses
    - `require.Arch(t *testing.T, arch string)`: only run the current test if
      the CPU architecture we're currently running on is `arch`
    - `require.ArchIn(t *testing.T, archs []string)`: only run the current test
      if the CPU architecture we're currently running on is among those listed
      in `archs`
  - See internal/pkg/test/tool/require/require.go for the full set of
    require.XYZ() functions.

- `user.CurrentUser(t *testing.T) *user.User`, defined in
  [e2e/internal/e2e/user.go](https://github.com/apptainer/apptainer/blob/main/e2e/internal/e2e/user.go),
  returns a struct with information about the current user (UID, GID, home
  directory, etc.)
  - The `user.User` struct is defined in internal/pkg/util/user/identity_unix.go

## Common pitfalls

### Test not visible in logs

**Scenario:** You've written your new test function, placed it in the right
file (e.g. `imgbuild.go`), and now... your test doesn't seem to be running. You
can't find it in the e2e output logs anywhere.

**Common cause:** You've forgotten to add your test function to the
`testhelper.Tests` map created by your testing group's E2ETests() function.

This function is typically located at the bottom of the Go source file
corresponding to the e2e group.
Your test function - really, a method of the group's e2e context
object - won't run unless it is added to the `testhelper.Tests` map.

Make sure to also [mark your test
appropriately](#parallel-par-vs-non-parallel-seq-tests) if it cannot be run in
parallel.

## Running the e2e suite

To run the e2e-test suite, use the `e2e-test` script:

```sh
./scripts/e2e-test -v
```

The `-v` option means verbose so it will list all the tests being run.

### Test filtering

To limit tests to a subset, use the `-run` option to the script with
a regular expression matching the paths of tests to run.

For example, to run only the tests in the ACTIONS group:

```sh
./scripts/e2e-test -v -run TestE2E/PAR/ACTIONS
```

or to run only sequential tests that include the word "global":

```sh
./scripts/e2e-test -v -run TestE2E/SEQ/.*global.*
```

It's not always easy to figure out what the full path of tests of interest.
The easiest way is to look at the log of an `e2e_tests` check on any
https://github.com/apptainer/apptainer pull request.

### Dealing with Docker Hub registry limits

The test suite is heavily relying on Docker Hub registry, since the introduction
of the rate pull limit, developers can quickly hit the quota limit leading to
the e2e tests randomly failed.

There is two possible approaches to minimize/avoid that:

1. if you have an account on Docker Hub you can specify and export your
credentials via environment variables `E2E_DOCKER_USERNAME` and
`E2E_DOCKER_PASSWORD` before running the test suite, however if you have
a free account the quota limit is simply doubled and may not work for you
2. or you can run a local pull through cache registry and use
`E2E_DOCKER_MIRROR`/`E2E_DOCKER_MIRROR_INSECURE` environment variables

### Run a local pull through cache registry

The most straightforward way to run it is to run in a terminal:

```sh
mkdir -p $HOME/.cache/registry
apptainer run --env REGISTRY_HTTP_ADDR=127.0.0.1:5001 \
                --env REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
                --bind $HOME/.cache/registry:/var/lib/registry \
                docker://registry:2.7
```

And run the test suite in another terminal:

```sh
export E2E_DOCKER_MIRROR=127.0.0.1:5001
export E2E_DOCKER_MIRROR_INSECURE=true
./scripts/e2e-test -v
```
