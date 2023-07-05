// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/interactive"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	ocitypes "github.com/containers/image/v5/types"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var buildArgs struct {
	sections            []string
	bindPaths           []string
	mounts              []string
	libraryURL          string
	keyServerURL        string
	webURL              string
	encrypt             bool
	fakeroot            bool
	fixPerms            bool
	isJSON              bool
	noCleanUp           bool
	noTest              bool
	sandbox             bool
	update              bool
	nvidia              bool
	nvccli              bool
	rocm                bool
	writableTmpfs       bool     // For test section only
	userns              bool     // Enable user namespaces
	ignoreSubuid        bool     // Ignore /etc/subuid entries (hidden)
	ignoreFakerootCmd   bool     // Ignore fakeroot command (hidden)
	ignoreUserns        bool     // Ignore user namespace(hidden)
	remote              bool     // Remote flag(hidden, only for helpful error message)
	buildVarArgs        []string // Variables passed to build procedure.
	buildVarArgFile     string   // Variables file passed to build procedure.
	buildArgsUnusedWarn bool     // Variables passed to build procedure to turn fatal error to warn.
}

// -s|--sandbox
var buildSandboxFlag = cmdline.Flag{
	ID:           "buildSandboxFlag",
	Value:        &buildArgs.sandbox,
	DefaultValue: false,
	Name:         "sandbox",
	ShortHand:    "s",
	Usage:        "build image as sandbox format (chroot directory structure)",
	EnvKeys:      []string{"SANDBOX"},
}

// --section
var buildSectionFlag = cmdline.Flag{
	ID:           "buildSectionFlag",
	Value:        &buildArgs.sections,
	DefaultValue: []string{"all"},
	Name:         "section",
	Usage:        "only run specific section(s) of deffile (setup, post, files, environment, test, labels, none)",
	EnvKeys:      []string{"SECTION"},
}

// --json
var buildJSONFlag = cmdline.Flag{
	ID:           "buildJSONFlag",
	Value:        &buildArgs.isJSON,
	DefaultValue: false,
	Name:         "json",
	Usage:        "interpret build definition as JSON",
	EnvKeys:      []string{"JSON"},
}

// -u|--update
var buildUpdateFlag = cmdline.Flag{
	ID:           "buildUpdateFlag",
	Value:        &buildArgs.update,
	DefaultValue: false,
	Name:         "update",
	ShortHand:    "u",
	Usage:        "run definition over existing container (skips header)",
	EnvKeys:      []string{"UPDATE"},
}

// -T|--notest
var buildNoTestFlag = cmdline.Flag{
	ID:           "buildNoTestFlag",
	Value:        &buildArgs.noTest,
	DefaultValue: false,
	Name:         "notest",
	ShortHand:    "T",
	Usage:        "build without running tests in %test section",
	EnvKeys:      []string{"NOTEST"},
}

// --library
var buildLibraryFlag = cmdline.Flag{
	ID:           "buildLibraryFlag",
	Value:        &buildArgs.libraryURL,
	DefaultValue: "",
	Name:         "library",
	Usage:        "container Library URL",
	EnvKeys:      []string{"LIBRARY"},
}

// --disable-cache
var buildDisableCacheFlag = cmdline.Flag{
	ID:           "buildDisableCacheFlag",
	Value:        &disableCache,
	DefaultValue: false,
	Name:         "disable-cache",
	Usage:        "do not use cache or create cache",
	EnvKeys:      []string{"DISABLE_CACHE"},
}

// --no-cleanup
var buildNoCleanupFlag = cmdline.Flag{
	ID:           "buildNoCleanupFlag",
	Value:        &buildArgs.noCleanUp,
	DefaultValue: false,
	Name:         "no-cleanup",
	Usage:        "do NOT clean up bundle after failed build, can be helpful for debugging",
	EnvKeys:      []string{"NO_CLEANUP"},
}

// --fakeroot
var buildFakerootFlag = cmdline.Flag{
	ID:           "buildFakerootFlag",
	Value:        &buildArgs.fakeroot,
	DefaultValue: false,
	Name:         "fakeroot",
	ShortHand:    "f",
	Usage:        "build with the appearance of running as root (default when building from a definition file unprivileged)",
	EnvKeys:      []string{"FAKEROOT"},
}

// -e|--encrypt
var buildEncryptFlag = cmdline.Flag{
	ID:           "buildEncryptFlag",
	Value:        &buildArgs.encrypt,
	DefaultValue: false,
	Name:         "encrypt",
	ShortHand:    "e",
	Usage:        "build an image with an encrypted file system",
}

// TODO: Deprecate at 3.6, remove at 3.8
// --fix-perms
var buildFixPermsFlag = cmdline.Flag{
	ID:           "fixPermsFlag",
	Value:        &buildArgs.fixPerms,
	DefaultValue: false,
	Name:         "fix-perms",
	Usage:        "ensure owner has rwX permissions on all container content for oci/docker sources",
	EnvKeys:      []string{"FIXPERMS"},
}

// --nv
var buildNvFlag = cmdline.Flag{
	ID:           "nvFlag",
	Value:        &buildArgs.nvidia,
	DefaultValue: false,
	Name:         "nv",
	Usage:        "inject host Nvidia libraries during build for post and test sections",
}

// --nvccli
var buildNvCCLIFlag = cmdline.Flag{
	ID:           "buildNvCCLIFlag",
	Value:        &buildArgs.nvccli,
	DefaultValue: false,
	Name:         "nvccli",
	Usage:        "use nvidia-container-cli for GPU setup (experimental)",
	EnvKeys:      []string{"NVCCLI"},
}

// --rocm
var buildRocmFlag = cmdline.Flag{
	ID:           "rocmFlag",
	Value:        &buildArgs.rocm,
	DefaultValue: false,
	Name:         "rocm",
	Usage:        "inject host Rocm libraries during build for post and test sections",
}

// -B|--bind
var buildBindFlag = cmdline.Flag{
	ID:           "buildBindFlag",
	Value:        &buildArgs.bindPaths,
	DefaultValue: cmdline.StringArray{}, // to allow commas in bind path
	Name:         "bind",
	ShortHand:    "B",
	Usage: "a user-bind path specification. spec has the format src[:dest[:opts]]," +
		"where src and dest are outside and inside paths. If dest is not given," +
		"it is set equal to src. Mount options ('opts') may be specified as 'ro'" +
		"(read-only) or 'rw' (read/write, which is the default)." +
		"Multiple bind paths can be given by a comma separated list.",
}

// --mount
var buildMountFlag = cmdline.Flag{
	ID:           "buildMountFlag",
	Value:        &buildArgs.mounts,
	DefaultValue: cmdline.StringArray{}, // to allow commas in bind path
	Name:         "mount",
	Usage:        "a mount specification e.g. 'type=bind,source=/opt,destination=/hostopt'.",
	EnvKeys:      []string{"MOUNT"},
	Tag:          "<spec>",
	EnvHandler:   cmdline.EnvAppendValue,
}

// --writable-tmpfs
var buildWritableTmpfsFlag = cmdline.Flag{
	ID:           "buildWritableTmpfsFlag",
	Value:        &buildArgs.writableTmpfs,
	DefaultValue: false,
	Name:         "writable-tmpfs",
	Usage:        "during the %test section, makes the file system accessible as read-write with non persistent data (with overlay support only)",
	EnvKeys:      []string{"WRITABLE_TMPFS"},
}

// --userns
var buildUsernsFlag = cmdline.Flag{
	ID:           "buildUsernsFlag",
	Value:        &buildArgs.userns,
	DefaultValue: false,
	Name:         "userns",
	Usage:        "build without using setuid even if available",
	EnvKeys:      []string{"USERNS"},
}

// --ignore-subuid
var buildIgnoreSubuidFlag = cmdline.Flag{
	ID:           "buildIgnoreSubuidFlag",
	Value:        &buildArgs.ignoreSubuid,
	DefaultValue: false,
	Name:         "ignore-subuid",
	Usage:        "ignore entries inside /etc/subuid",
	EnvKeys:      []string{"IGNORE_SUBUID"},
	Hidden:       true,
}

// --ignore-fakeroot-command
var buildIgnoreFakerootCommand = cmdline.Flag{
	ID:           "buildIgnoreFakerootCommandFlag",
	Value:        &buildArgs.ignoreFakerootCmd,
	DefaultValue: false,
	Name:         "ignore-fakeroot-command",
	Usage:        "ignore fakeroot command",
	EnvKeys:      []string{"IGNORE_FAKEROOT_COMMAND"},
	Hidden:       true,
}

// --ignore-userns
var buildIgnoreUsernsFlag = cmdline.Flag{
	ID:           "buildIgnoreUsernsFlag",
	Value:        &buildArgs.ignoreUserns,
	DefaultValue: false,
	Name:         "ignore-userns",
	Usage:        "ignore user namespaces",
	EnvKeys:      []string{"IGNORE_USERNS"},
	Hidden:       true,
}

var buildRemoteFlag = cmdline.Flag{
	ID:           "remoteFlag",
	Value:        &buildArgs.remote,
	DefaultValue: false,
	Name:         "remote",
	Usage:        "--remote is no longer supported, try building locally without it",
	EnvKeys:      []string{},
	Hidden:       true,
}

// --build-arg
var buildVarArgsFlag = cmdline.Flag{
	ID:           "buildVarArgsFlag",
	Value:        &buildArgs.buildVarArgs,
	DefaultValue: []string{},
	Name:         "build-arg",
	Usage:        "defines variable=value to replace {{ variable }} entries in build definition file",
}

// --build-arg-file
var buildVarArgFileFlag = cmdline.Flag{
	ID:           "buildVarArgFileFlag",
	Value:        &buildArgs.buildVarArgFile,
	DefaultValue: "",
	Name:         "build-arg-file",
	Usage:        "specifies a file containing variable=value lines to replace '{{ variable }}' with value in build definition files",
}

var buildArgUnusedWarn = cmdline.Flag{
	ID:           "buildArgUnusedWarnFlag",
	Value:        &buildArgs.buildArgsUnusedWarn,
	DefaultValue: false,
	Name:         "warn-unused-build-args",
	Usage:        "shows warning instead of fatal message when build args are not exact matched",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(buildCmd)

		cmdManager.RegisterFlagForCmd(&buildDisableCacheFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildEncryptFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildFakerootFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildFixPermsFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildJSONFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildLibraryFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildNoCleanupFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildNoTestFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildSandboxFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildSectionFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildUpdateFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&commonForceFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&commonNoHTTPSFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&commonTmpDirFlag, buildCmd)

		cmdManager.RegisterFlagForCmd(&dockerHostFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&dockerUsernameFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&dockerPasswordFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&dockerLoginFlag, buildCmd)

		cmdManager.RegisterFlagForCmd(&commonPromptForPassphraseFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&commonPEMFlag, buildCmd)

		cmdManager.RegisterFlagForCmd(&buildNvFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildNvCCLIFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildRocmFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildBindFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildMountFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildWritableTmpfsFlag, buildCmd)

		cmdManager.RegisterFlagForCmd(&buildUsernsFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildIgnoreSubuidFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildIgnoreFakerootCommand, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildIgnoreUsernsFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildRemoteFlag, buildCmd)

		cmdManager.RegisterFlagForCmd(&buildVarArgsFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildVarArgFileFlag, buildCmd)
		cmdManager.RegisterFlagForCmd(&buildArgUnusedWarn, buildCmd)
	})
}

// buildCmd represents the build command.
var buildCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),

	Use:              docs.BuildUse,
	Short:            docs.BuildShort,
	Long:             docs.BuildLong,
	Example:          docs.BuildExample,
	PreRun:           preRun,
	Run:              runBuild,
	TraverseChildren: true,
}

func preRun(cmd *cobra.Command, args []string) {
	spec := args[len(args)-1]
	isDeffile := fs.IsFile(spec) && !isImage(spec)
	if buildArgs.fakeroot {
		fakerootExec(isDeffile, false)
	} else {
		if os.Getuid() != 0 {
			if isDeffile {
				sylog.Verbosef("Implying --fakeroot because building from definition file unprivileged")
				fakerootExec(isDeffile, true)
			} else if buildArgs.encrypt {
				sylog.Verbosef("Implying --fakeroot because using unprivileged encryption")
				fakerootExec(isDeffile, true)
			}
		}
	}

	if buildArgs.remote {
		err := errors.New("--remote is no longer supported, try building locally without it")
		cobra.CheckErr(err)
	}
}

// checkBuildTarget makes sure output target doesn't exist, or is ok to overwrite.
// And checks that update flag will update an existing directory.
func checkBuildTarget(path string) error {
	abspath, err := fs.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %q: %v", path, err)
	}

	if !buildArgs.sandbox && buildArgs.update {
		return fmt.Errorf("only sandbox update is supported: --sandbox flag is missing")
	}
	if f, err := os.Stat(abspath); err == nil {
		if buildArgs.update && !f.IsDir() {
			return fmt.Errorf("only sandbox update is supported: %s is not a directory", abspath)
		}
		// check if the sandbox image being overwritten looks like an Apptainer
		// image and inform users to check its content and use --force option if
		// the sandbox image is not an Apptainer image
		if f.IsDir() && !forceOverwrite {
			files, err := os.ReadDir(abspath)
			if err != nil {
				return fmt.Errorf("could not read sandbox directory %s: %s", abspath, err)
			} else if len(files) > 0 {
				required := 0
				for _, f := range files {
					switch f.Name() {
					case ".singularity.d", "dev", "proc", "sys":
						required++
					}
				}
				if required != 4 {
					return fmt.Errorf("%s is not empty and is not an Apptainer sandbox, check its content first and use --force if you want to overwrite it", abspath)
				}
			}
		}
		if !buildArgs.update && !forceOverwrite {
			// If non-interactive, die... don't try to prompt the user y/n
			if !term.IsTerminal(syscall.Stdin) {
				return fmt.Errorf("build target '%s' already exists. Use --force if you want to overwrite it", f.Name())
			}

			question := fmt.Sprintf("Build target '%s' already exists and will be deleted during the build process. Do you want to continue? [y/N] ", f.Name())

			img, err := image.Init(abspath, false)
			if err != nil {
				if err != image.ErrUnknownFormat {
					return fmt.Errorf("while determining '%s' format: %s", f.Name(), err)
				}
				// unknown image file format
				question = fmt.Sprintf("Build target '%s' may be a definition file or a text/binary file that will be overwritten. Do you still want to overwrite it? [y/N] ", f.Name())
			} else {
				img.File.Close()
			}

			input, err := interactive.AskYNQuestion("n", question)
			if err != nil {
				return fmt.Errorf("while reading the input: %s", err)
			}
			if input != "y" {
				return fmt.Errorf("stopping build")
			}
			forceOverwrite = true
		}
	} else if os.IsNotExist(err) && buildArgs.update && buildArgs.sandbox {
		return fmt.Errorf("could not update sandbox %s: doesn't exist", abspath)
	}
	return nil
}

// makeDockerCredentials creates an *ocitypes.DockerAuthConfig to use for
// OCI/Docker registry operation configuration. Note that if we don't have a
// username or password set it will return a nil pointer, as containers/image
// requires this to fall back to .docker/config based authentication.
func makeDockerCredentials(cmd *cobra.Command) (authConf *ocitypes.DockerAuthConfig, err error) {
	usernameFlag := cmd.Flags().Lookup("docker-username")
	passwordFlag := cmd.Flags().Lookup("docker-password")

	if dockerLogin {
		if !usernameFlag.Changed {
			dockerAuthConfig.Username, err = interactive.AskQuestion("Enter Docker Username: ")
			if err != nil {
				return authConf, err
			}
			usernameFlag.Value.Set(dockerAuthConfig.Username)
			usernameFlag.Changed = true
		}

		dockerAuthConfig.Password, err = interactive.AskQuestionNoEcho("Enter Docker Password: ")
		if err != nil {
			return authConf, err
		}
		passwordFlag.Value.Set(dockerAuthConfig.Password)
		passwordFlag.Changed = true
	}

	if usernameFlag.Changed || passwordFlag.Changed {
		return &dockerAuthConfig, nil
	}

	// If a username / password have not been explicitly set, return a nil
	// pointer, which will mean containers/image falls back to looking for
	// .docker/config.json
	return nil, nil
}
