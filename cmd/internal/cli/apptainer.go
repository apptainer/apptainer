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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/plugin"
	"github.com/apptainer/apptainer/internal/pkg/remote"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/cmdline"
	clicallback "github.com/apptainer/apptainer/pkg/plugin/callback/cli"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	keyClient "github.com/apptainer/container-key-client/client"
	libClient "github.com/apptainer/container-library-client/client"
	ocitypes "github.com/containers/image/v5/types"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// cmdInits holds all the init function to be called
// for commands/flags registration.
var cmdInits = make([]func(*cmdline.CommandManager), 0)

// CurrentUser holds the current user account information
var CurrentUser = getCurrentUser()

// currentRemoteEndpoint holds the current remote endpoint
var currentRemoteEndpoint *endpoint.Config

var (
	dockerAuthConfig ocitypes.DockerAuthConfig
	dockerLogin      bool
	dockerHost       string

	encryptionPEMPath   string
	promptForPassphrase bool
	forceOverwrite      bool
	noHTTPS             bool
	useBuildConfig      bool
	tmpDir              string
)

// apptainer command flags
var (
	debug   bool
	nocolor bool
	silent  bool
	verbose bool
	quiet   bool

	configurationFile string
)

// -d|--debug
var singDebugFlag = cmdline.Flag{
	ID:           "singDebugFlag",
	Value:        &debug,
	DefaultValue: false,
	Name:         "debug",
	ShortHand:    "d",
	Usage:        "print debugging information (highest verbosity)",
	EnvKeys:      []string{"DEBUG"},
}

// --nocolor
var singNoColorFlag = cmdline.Flag{
	ID:           "singNoColorFlag",
	Value:        &nocolor,
	DefaultValue: false,
	Name:         "nocolor",
	Usage:        "print without color output (default False)",
}

// -s|--silent
var singSilentFlag = cmdline.Flag{
	ID:           "singSilentFlag",
	Value:        &silent,
	DefaultValue: false,
	Name:         "silent",
	ShortHand:    "s",
	Usage:        "only print errors",
}

// -q|--quiet
var singQuietFlag = cmdline.Flag{
	ID:           "singQuietFlag",
	Value:        &quiet,
	DefaultValue: false,
	Name:         "quiet",
	ShortHand:    "q",
	Usage:        "suppress normal output",
}

// -v|--verbose
var singVerboseFlag = cmdline.Flag{
	ID:           "singVerboseFlag",
	Value:        &verbose,
	DefaultValue: false,
	Name:         "verbose",
	ShortHand:    "v",
	Usage:        "print additional information",
}

// --docker-username
var dockerUsernameFlag = cmdline.Flag{
	ID:            "dockerUsernameFlag",
	Value:         &dockerAuthConfig.Username,
	DefaultValue:  "",
	Name:          "docker-username",
	Usage:         "specify a username for docker authentication",
	Hidden:        true,
	EnvKeys:       []string{"DOCKER_USERNAME"},
	WithoutPrefix: true,
}

// --docker-password
var dockerPasswordFlag = cmdline.Flag{
	ID:            "dockerPasswordFlag",
	Value:         &dockerAuthConfig.Password,
	DefaultValue:  "",
	Name:          "docker-password",
	Usage:         "specify a password for docker authentication",
	Hidden:        true,
	EnvKeys:       []string{"DOCKER_PASSWORD"},
	WithoutPrefix: true,
}

// --docker-login
var dockerLoginFlag = cmdline.Flag{
	ID:           "dockerLoginFlag",
	Value:        &dockerLogin,
	DefaultValue: false,
	Name:         "docker-login",
	Usage:        "login to a Docker Repository interactively",
	EnvKeys:      []string{"DOCKER_LOGIN"},
}

// --docker-host
var dockerHostFlag = cmdline.Flag{
	ID:            "dockerHostFlag",
	Value:         &dockerHost,
	DefaultValue:  "",
	Name:          "docker-host",
	Usage:         "specify a custom Docker daemon host",
	EnvKeys:       []string{"DOCKER_HOST"},
	WithoutPrefix: true,
}

// --passphrase
var commonPromptForPassphraseFlag = cmdline.Flag{
	ID:           "commonPromptForPassphraseFlag",
	Value:        &promptForPassphrase,
	DefaultValue: false,
	Name:         "passphrase",
	Usage:        "prompt for an encryption passphrase",
}

// --pem-path
var commonPEMFlag = cmdline.Flag{
	ID:           "actionEncryptionPEMPath",
	Value:        &encryptionPEMPath,
	DefaultValue: "",
	Name:         "pem-path",
	Usage:        "enter an path to a PEM formatted RSA key for an encrypted container",
}

// -F|--force
var commonForceFlag = cmdline.Flag{
	ID:           "commonForceFlag",
	Value:        &forceOverwrite,
	DefaultValue: false,
	Name:         "force",
	ShortHand:    "F",
	Usage:        "overwrite an image file if it exists",
	EnvKeys:      []string{"FORCE"},
}

// --no-https
var commonNoHTTPSFlag = cmdline.Flag{
	ID:           "commonNoHTTPSFlag",
	Value:        &noHTTPS,
	DefaultValue: false,
	Name:         "no-https",
	Usage:        "use http instead of https for docker:// oras:// and library://<hostname>/... URIs",
	EnvKeys:      []string{"NOHTTPS", "NO_HTTPS"},
}

// --nohttps (deprecated)
var commonOldNoHTTPSFlag = cmdline.Flag{
	ID:           "commonOldNoHTTPSFlag",
	Value:        &noHTTPS,
	DefaultValue: false,
	Name:         "nohttps",
	Deprecated:   "use --no-https",
	Usage:        "use http instead of https for docker:// oras:// and library://<hostname>/... URIs",
}

// --tmpdir
var commonTmpDirFlag = cmdline.Flag{
	ID:           "commonTmpDirFlag",
	Value:        &tmpDir,
	DefaultValue: os.TempDir(),
	Hidden:       true,
	Name:         "tmpdir",
	Usage:        "specify a temporary directory to use for build",
	EnvKeys:      []string{"TMPDIR"},
}

// -c|--config
var singConfigFileFlag = cmdline.Flag{
	ID:           "singConfigFileFlag",
	Value:        &configurationFile,
	DefaultValue: buildcfg.APPTAINER_CONF_FILE,
	Name:         "config",
	ShortHand:    "c",
	Usage:        "specify a configuration file (for root or unprivileged installation only)",
	EnvKeys:      []string{"CONFIG_FILE"},
}

// --build-config
var singBuildConfigFlag = cmdline.Flag{
	ID:           "singBuildConfigFlag",
	Value:        &useBuildConfig,
	DefaultValue: false,
	Name:         "build-config",
	Usage:        "use configuration needed for building containers",
}

func getCurrentUser() *user.User {
	usr, err := user.Current()
	if err != nil {
		sylog.Fatalf("Couldn't determine user account information: %v", err)
	}
	return usr
}

func addCmdInit(cmdInit func(*cmdline.CommandManager)) {
	cmdInits = append(cmdInits, cmdInit)
}

func setSylogMessageLevel() {
	var level int

	if debug {
		level = 5
		// Propagate debug flag to nested `apptainer` calls.
		os.Setenv("APPTAINER_DEBUG", "1")
	} else if verbose {
		level = 4
	} else if quiet {
		level = -1
	} else if silent {
		level = -3
	} else {
		level = 1
	}

	color := true
	if nocolor || !term.IsTerminal(2) {
		color = false
	}

	sylog.SetLevel(level, color)
}

// handleRemoteConf will make sure your 'remote.yaml' config file
// has the correct permission.
func handleRemoteConf(remoteConfFile string) error {
	// Only check the permission if it exists.
	if fs.IsFile(remoteConfFile) {
		sylog.Debugf("Ensuring file permission of 0600 on %s", remoteConfFile)
		if err := fs.EnsureFileWithPermission(remoteConfFile, 0o600); err != nil {
			return fmt.Errorf("unable to correct the permission on %s: %w", remoteConfFile, err)
		}
	}
	return nil
}

// handleConfDir tries to create the user's configuration directory and handles
// messages and/or errors.
func handleConfDir(confDir, legacyConfigDir string) {
	ok, err := fs.PathExists(confDir)
	if err != nil {
		sylog.Warningf("Unable to retrieve information for %s: %s", confDir, err)
		return
	}

	// apptainer user config directory exists, run perm check and return
	if ok {
		sylog.Debugf("%s already exists. Not creating.", confDir)
		fi, err := os.Stat(confDir)
		if err != nil {
			sylog.Warningf("Unable to retrieve information for %s: %s", confDir, err)
			return
		}
		if fi.Mode().Perm() != 0o700 {
			sylog.Debugf("Enforce permission 0700 on %s", confDir)
			// enforce permission on user configuration directory
			if err := os.Chmod(confDir, 0o700); err != nil {
				// best effort as chmod could fail for various reasons (eg: readonly FS)
				sylog.Warningf("Couldn't enforce permission 0700 on %s: %s", confDir, err)
			}
		}
		return
	}

	// apptainer user config directory doesnt exist, create it.
	err = fs.Mkdir(confDir, 0o700)
	if err != nil {
		sylog.Debugf("Could not create %s: %s", confDir, err)
		return
	}

	sylog.Debugf("Created %s", confDir)

	// check if singularity user config directory exists, use it to populate configs of
	// apptainer user config directory if it does.
	ok, err = fs.PathExists(legacyConfigDir)
	if err != nil {
		sylog.Warningf("Failed to retrieve information for %s: %s", legacyConfigDir, err)
		return
	}

	// singularity user config directory doesnt exist, return
	if !ok {
		return
	}

	sylog.Infof("Detected Singularity user configuration directory")

	migrateRemoteConf(confDir, legacyConfigDir)
	migrateDockerConf(confDir, legacyConfigDir)
	migrateKeys(confDir, legacyConfigDir)
}

func migrateRemoteConf(confDir, legacyConfigDir string) {
	ok, err := fs.PathExists(syfs.LegacyRemoteConf())
	if err != nil {
		sylog.Warningf("Failed to retrieve information for %s: %s", syfs.LegacyRemoteConf(), err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", syfs.LegacyRemoteConf(), syfs.RemoteConf())
		return
	} else if !ok {
		return
	}

	sylog.Infof("Detected Singularity remote configuration, migrating...")

	// Try to load legacy remote config to check version compatibility
	_, err = loadRemoteConf(syfs.LegacyRemoteConf())
	if err != nil {
		sylog.Warningf("Migration failed, unable to read legacy remote configuration: %s", err)
		sylog.Warningf("It may be of an incompatible format and needs to be reconstructed manually with the \"apptainer remote\" command group.")
		return
	}

	err = fs.CopyFile(syfs.LegacyRemoteConf(), syfs.RemoteConf(), 0o600)
	if err != nil {
		sylog.Warningf("Failed to migrate %s to %s: %s", syfs.LegacyRemoteConf(), syfs.RemoteConf(), err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", syfs.LegacyRemoteConf(), syfs.RemoteConf())
	}
}

func migrateDockerConf(confDir, legacyConfigDir string) {
	ok, err := fs.PathExists(syfs.LegacyDockerConf())
	if err != nil {
		sylog.Warningf("Failed to retrieve information for %s: %s", syfs.LegacyDockerConf(), err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", syfs.LegacyDockerConf(), syfs.DockerConf())
		return
	} else if !ok {
		return
	}

	sylog.Infof("Detected Singularity docker configuration, migrating...")
	err = fs.CopyFile(syfs.LegacyDockerConf(), syfs.DockerConf(), 0o600)
	if err != nil {
		sylog.Warningf("Failed to migrate %s to %s: %s", syfs.LegacyDockerConf(), syfs.DockerConf(), err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", syfs.LegacyDockerConf(), syfs.DockerConf())
	}
}

func migrateKeys(confDir, legacyConfigDir string) {
	legacySypgpDir := filepath.Join(syfs.LegacyConfigDir(), sypgp.LegacyDirectory)
	keysDir := filepath.Join(syfs.ConfigDir(), sypgp.Directory)
	ok, err := fs.PathExists(legacySypgpDir)
	if err != nil {
		sylog.Warningf("Failed to retrieve information for %s: %s", legacySypgpDir, err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", legacySypgpDir, keysDir)
		return
	} else if !ok {
		return
	}

	err = fs.Mkdir(keysDir, 0o700)
	if err != nil {
		sylog.Debugf("Could not create %s: %s", keysDir, err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", legacySypgpDir, keysDir)
		return
	}

	migrateGPGPublic(keysDir, legacySypgpDir)
	migrateGPGPrivate(keysDir, legacySypgpDir)
}

func migrateGPGPublic(sypgpDir, legacySypgpDir string) {
	legacyPublicPath := filepath.Join(legacySypgpDir, sypgp.PublicFile)
	publicPath := filepath.Join(sypgpDir, sypgp.PublicFile)
	ok, err := fs.PathExists(legacyPublicPath)
	if err != nil {
		sylog.Warningf("Failed to retrieve information for %s: %s", legacyPublicPath, err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", legacyPublicPath, publicPath)
		return
	} else if !ok {
		return
	}

	sylog.Infof("Detected public Singularity pgp keyring, migrating...")
	err = fs.CopyFile(legacyPublicPath, publicPath, 0o600)
	if err != nil {
		sylog.Warningf("Failed to migrate %s to %s: %s", legacyPublicPath, publicPath, err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", legacyPublicPath, publicPath)
	}
}

func migrateGPGPrivate(sypgpDir, legacySypgpDir string) {
	legacyPrivatePath := filepath.Join(legacySypgpDir, sypgp.SecretFile)
	privatePath := filepath.Join(sypgpDir, sypgp.SecretFile)
	ok, err := fs.PathExists(legacyPrivatePath)
	if err != nil {
		sylog.Warningf("Failed to retrieve information for %s: %s", legacyPrivatePath, err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", legacyPrivatePath, privatePath)
		return
	} else if !ok {
		return
	}

	sylog.Infof("Detected private Singularity pgp keyring, migrating...")
	err = fs.CopyFile(legacyPrivatePath, privatePath, 0o600)
	if err != nil {
		sylog.Warningf("Failed to migrate %s to %s: %s", legacyPrivatePath, privatePath, err)
		sylog.Warningf("Migration failed, you can migrate manually with \"cp -a %s %s\"", legacyPrivatePath, privatePath)
	}
}

func persistentPreRun(cmd *cobra.Command, args []string) error {
	setSylogMessageLevel()
	sylog.Debugf("Apptainer version: %s", buildcfg.PACKAGE_VERSION)

	if cmd.CalledAs() == "confgen" {
		// This command generates the configuration so it may
		// not yet be there
		return nil
	}

	var config *apptainerconf.File
	var err error
	if useBuildConfig {
		sylog.Debugf("Using container build configuration")
		// Base this on a default configuration.
		config, err = apptainerconf.Parse("")
		if err != nil {
			return fmt.Errorf("failure getting default config: %v", err)
		}
		apptainerconf.ApplyBuildConfig(config)
	} else {
		if os.Geteuid() != 0 && buildcfg.APPTAINER_SUID_INSTALL == 1 {
			if configurationFile != singConfigFileFlag.DefaultValue {
				return fmt.Errorf("--config requires to be root or an unprivileged installation")
			}
		}

		oldconfdir := filepath.Dir(filepath.Dir(configurationFile)) + "/singularity/"

		if _, err := os.Stat(oldconfdir); err == nil {
			sylog.Infof("%s exists; cleanup by system administrator is not complete (see https://apptainer.org/docs/admin/latest/singularity_migration.html)", oldconfdir)
		}

		sylog.Debugf("Parsing configuration file %s", configurationFile)
		config, err = apptainerconf.Parse(configurationFile)
		if err != nil {
			return fmt.Errorf("couldn't parse configuration file %s: %s", configurationFile, err)
		}
	}
	apptainerconf.SetCurrentConfig(config)
	// Include the user's PATH for now.
	// It will be overridden later if using setuid flow.
	apptainerconf.SetBinaryPath(buildcfg.LIBEXECDIR, true)

	// Handle the config dir (~/.apptainer),
	// then check the remove conf file permission.
	handleConfDir(syfs.ConfigDir(), syfs.LegacyConfigDir())
	if err := handleRemoteConf(syfs.RemoteConf()); err != nil {
		return fmt.Errorf("while handling remote config: %w", err)
	}
	return nil
}

// Init initializes and registers all apptainer commands.
func Init(loadPlugins bool) {
	cmdManager := cmdline.NewCommandManager(apptainerCmd)

	apptainerCmd.Flags().SetInterspersed(false)
	apptainerCmd.PersistentFlags().SetInterspersed(false)

	templateFuncs := template.FuncMap{
		"TraverseParentsUses": TraverseParentsUses,
	}
	cobra.AddTemplateFuncs(templateFuncs)

	apptainerCmd.SetHelpTemplate(docs.HelpTemplate)
	apptainerCmd.SetUsageTemplate(docs.UseTemplate)

	vt := fmt.Sprintf("%s version {{printf \"%%s\" .Version}}\n", buildcfg.PACKAGE_NAME)
	apptainerCmd.SetVersionTemplate(vt)

	// set persistent pre run function here to avoid initialization loop error
	apptainerCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		var err error
		foundKeys := make(map[string]string)
		for precedence := range env.ApptainerPrefixes {
			err = cmdManager.UpdateCmdFlagFromEnv(apptainerCmd, precedence, foundKeys)
			if nil != err {
				sylog.Fatalf("While parsing global environment variables: %s", err)
			}
		}
		for precedence := range env.ApptainerPrefixes {
			err = cmdManager.UpdateCmdFlagFromEnv(cmd, precedence, foundKeys)
			if nil != err {
				sylog.Fatalf("While parsing environment variables: %s", err)
			}
		}
		if err := persistentPreRun(cmd, args); err != nil {
			sylog.Fatalf("While initializing: %s", err)
		}
		return nil
	}

	cmdManager.RegisterFlagForCmd(&singDebugFlag, apptainerCmd)
	cmdManager.RegisterFlagForCmd(&singNoColorFlag, apptainerCmd)
	cmdManager.RegisterFlagForCmd(&singSilentFlag, apptainerCmd)
	cmdManager.RegisterFlagForCmd(&singQuietFlag, apptainerCmd)
	cmdManager.RegisterFlagForCmd(&singVerboseFlag, apptainerCmd)
	cmdManager.RegisterFlagForCmd(&singConfigFileFlag, apptainerCmd)
	cmdManager.RegisterFlagForCmd(&singBuildConfigFlag, apptainerCmd)

	cmdManager.RegisterCmd(VersionCmd)

	// register all others commands/flags
	for _, cmdInit := range cmdInits {
		cmdInit(cmdManager)
	}

	// load plugins and register commands/flags if any
	if loadPlugins {
		callbackType := (clicallback.Command)(nil)
		callbacks, err := plugin.LoadCallbacks(callbackType)
		if err != nil {
			sylog.Fatalf("Failed to load plugins callbacks '%T': %s", callbackType, err)
		}
		for _, c := range callbacks {
			c.(clicallback.Command)(cmdManager)
		}
	}

	// any error reported by command manager is considered as fatal
	cliErrors := len(cmdManager.GetError())
	if cliErrors > 0 {
		for _, e := range cmdManager.GetError() {
			sylog.Errorf("%s", e)
		}
		sylog.Fatalf("CLI command manager reported %d error(s)", cliErrors)
	}
}

// apptainerCmd is the base command when called without any subcommands
var apptainerCmd = &cobra.Command{
	TraverseChildren:      true,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdline.CommandError("invalid command")
	},

	Use:           docs.ApptainerUse,
	Version:       buildcfg.PACKAGE_VERSION,
	Short:         docs.ApptainerShort,
	Long:          docs.ApptainerLong,
	Example:       docs.ApptainerExample,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// RootCmd returns the root apptainer cobra command.
func RootCmd() *cobra.Command {
	return apptainerCmd
}

// ExecuteApptainer adds all child commands to the root command and sets
// flags appropriately. This is called by main.main(). It only needs to happen
// once to the root command (apptainer).
func ExecuteApptainer() {
	loadPlugins := true

	// we avoid to load installed plugins to not double load
	// them during execution of plugin compile and plugin install
	args := os.Args
	if len(args) > 1 {
		loadPlugins = !strings.HasPrefix(args[1], "plugin")
	}

	Init(loadPlugins)

	// Setup a cancellable context that will trap Ctrl-C / SIGINT
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			sylog.Debugf("User requested cancellation with interrupt")
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := apptainerCmd.ExecuteContext(ctx); err != nil {
		// Find the subcommand to display more useful help, and the correct
		// subcommand name in messages - i.e. 'run' not 'apptainer'
		// This is required because we previously used ExecuteC that returns the
		// subcommand... but there is no ExecuteC that variant accepts a context.
		subCmd, _, subCmdErr := apptainerCmd.Find(args[1:])
		if subCmdErr != nil {
			apptainerCmd.Printf("Error: %v\n\n", subCmdErr)
		}

		name := subCmd.Name()
		switch err.(type) {
		case cmdline.FlagError:
			usage := subCmd.Flags().FlagUsagesWrapped(getColumns())
			apptainerCmd.Printf("Error for command %q: %s\n\n", name, err)
			apptainerCmd.Printf("Options for %s command:\n\n%s\n", name, usage)
		case cmdline.CommandError:
			apptainerCmd.Println(subCmd.UsageString())
		default:
			apptainerCmd.Printf("Error for command %q: %s\n\n", name, err)
			apptainerCmd.Println(subCmd.UsageString())
		}
		apptainerCmd.Printf("Run '%s --help' for more detailed usage information.\n",
			apptainerCmd.CommandPath())
		os.Exit(1)
	}
}

// GenBashCompletion writes the bash completion file to w.
func GenBashCompletion(w io.Writer, name string) error {
	Init(false)
	apptainerCmd.Use = name
	return apptainerCmd.GenBashCompletion(w)
}

// TraverseParentsUses walks the parent commands and outputs a properly formatted use string
func TraverseParentsUses(cmd *cobra.Command) string {
	if cmd.HasParent() {
		return TraverseParentsUses(cmd.Parent()) + cmd.Use + " "
	}

	return cmd.Use + " "
}

// VersionCmd displays installed apptainer version
var VersionCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildcfg.PACKAGE_VERSION)
	},

	Use:   "version",
	Short: "Show the version for Apptainer",
}

func loadRemoteConf(filepath string) (*remote.Config, error) {
	f, err := os.OpenFile(filepath, os.O_RDONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("while opening remote config file: %s", err)
	}
	defer f.Close()

	c, err := remote.ReadFrom(f)
	if err != nil {
		return nil, fmt.Errorf("while parsing remote config data: %s", err)
	}

	return c, nil
}

// getRemote returns the remote in use or an error
func getRemote() (*endpoint.Config, error) {
	var c *remote.Config

	// try to load both remotes, check for errors, sync if both exist,
	// if neither exist return errNoDefault to return to old auth behavior
	cSys, sysErr := loadRemoteConf(remote.SystemConfigPath)
	cUsr, usrErr := loadRemoteConf(syfs.RemoteConf())
	if sysErr != nil && usrErr != nil {
		return endpoint.DefaultEndpointConfig, nil
	} else if sysErr != nil {
		c = cUsr
	} else if usrErr != nil {
		c = cSys
	} else {
		// sync cUsr with system config cSys
		if err := cUsr.SyncFrom(cSys); err != nil {
			return nil, err
		}
		c = cUsr
	}

	ep, err := c.GetDefault()
	if err == remote.ErrNoDefault {
		// all remotes have been deleted, fix that by returning
		// the default remote endpoint to avoid side effects when
		// pulling from library
		if len(c.Remotes) == 0 {
			return endpoint.DefaultEndpointConfig, nil
		}
		// otherwise notify users about available endpoints and
		// invite them to select one of them
		help := "use 'apptainer remote use <endpoint>', available endpoints are: "
		endpoints := make([]string, 0, len(c.Remotes))
		for name := range c.Remotes {
			endpoints = append(endpoints, name)
		}
		help += strings.Join(endpoints, ", ")
		return nil, fmt.Errorf("no default endpoint set: %s", help)
	}

	return ep, err
}

func apptainerExec(image string, args []string) (string, error) {
	// Record from stdout and store as a string to return as the contents of the file.
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	abspath, err := filepath.Abs(image)
	if err != nil {
		return "", fmt.Errorf("while determining absolute path for %s: %v", image, err)
	}

	// re-use apptainer exec to grab image file content,
	// we reduce binds to the bare minimum with options below
	cmdArgs := []string{"exec", "--contain", "--no-home", "--no-nv", "--no-rocm", abspath}
	cmdArgs = append(cmdArgs, args...)

	apptainerCmd := filepath.Join(buildcfg.BINDIR, "apptainer")

	cmd := exec.Command(apptainerCmd, cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// move to the root to not bind the current working directory
	// while inspecting the image
	cmd.Dir = "/"

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("unable to process command: %s: error output:\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}

// CheckRoot ensures that a command is executed with root privileges.
func CheckRoot(cmd *cobra.Command, args []string) {
	if os.Geteuid() != 0 {
		sylog.Fatalf("%q command requires root privileges", cmd.CommandPath())
	}
}

// CheckRootOrUnpriv ensures that a command is executed with root
// privileges or that Apptainer is installed unprivileged.
func CheckRootOrUnpriv(cmd *cobra.Command, args []string) {
	if os.Geteuid() != 0 && buildcfg.APPTAINER_SUID_INSTALL == 1 {
		sylog.Fatalf("%q command requires root privileges or an unprivileged installation", cmd.CommandPath())
	}
}

// getKeyServerClientOpts returns client options for keyserver access.
// A "" value for uri will return client options for the current endpoint.
// A specified uri will return client options for that keyserver.
func getKeyserverClientOpts(uri string, op endpoint.KeyserverOp) ([]keyClient.Option, error) {
	if currentRemoteEndpoint == nil {
		var err error

		// if we can load config and if default endpoint is set, use that
		// otherwise fall back on regular authtoken and URI behavior
		currentRemoteEndpoint, err = getRemote()
		if err != nil {
			return nil, fmt.Errorf("unable to load remote configuration: %v", err)
		}
	}
	if currentRemoteEndpoint == endpoint.DefaultEndpointConfig {
		sylog.Warningf("No default remote in use, falling back to default keyserver: %s", endpoint.DefaultKeyserverURI)
	}

	return currentRemoteEndpoint.KeyserverClientOpts(uri, op)
}

// getLibraryClientConfig returns client config for library server access.
// A "" value for uri will return client config for the current endpoint.
// A specified uri will return client options for that library server.
func getLibraryClientConfig(uri string) (*libClient.Config, error) {
	if currentRemoteEndpoint == nil {
		var err error

		// if we can load config and if default endpoint is set, use that
		// otherwise fall back on regular authtoken and URI behavior
		currentRemoteEndpoint, err = getRemote()
		if err != nil {
			return nil, fmt.Errorf("unable to load remote configuration: %v", err)
		}
	}
	if currentRemoteEndpoint == endpoint.DefaultEndpointConfig {
		if endpoint.DefaultLibraryURI != "" {
			sylog.Warningf("no default remote in use, falling back to default library: %s", endpoint.DefaultLibraryURI)
		} else {
			return nil, fmt.Errorf("no default remote with library client in use (see https://apptainer.org/docs/user/latest/endpoint.html#no-default-remote)")
		}
	}

	libClientConfig, err := currentRemoteEndpoint.LibraryClientConfig(uri)
	if err != nil {
		return nil, err
	}
	if libClientConfig.BaseURL == "" {
		return nil, fmt.Errorf("remote has no library client")
	}
	return libClientConfig, nil
}
