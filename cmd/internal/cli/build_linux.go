// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"context"
	"fmt"
	"os"
	osExec "os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/build"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	fakerootConfig "github.com/apptainer/apptainer/internal/pkg/runtime/engine/fakeroot/config"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/interactive"
	"github.com/apptainer/apptainer/internal/pkg/util/starter"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/cryptkey"
	"github.com/spf13/cobra"
)

func fakerootExec(cmdArgs []string) {
	if buildArgs.nvccli && !buildArgs.noTest {
		sylog.Warningf("Due to writable-tmpfs limitations, %%test sections will fail with --nvccli & --fakeroot")
		sylog.Infof("Use -T / --notest to disable running tests during the build")
	}

	useSuid := buildcfg.APPTAINER_SUID_INSTALL == 1

	short := "-" + buildFakerootFlag.ShortHand
	long := "--" + buildFakerootFlag.Name
	envKey := fmt.Sprintf("APPTAINER_%s", buildFakerootFlag.EnvKeys[0])
	fakerootEnv := os.Getenv(envKey) != ""

	argsLen := len(os.Args) - 1
	if fakerootEnv {
		argsLen = len(os.Args)
		os.Unsetenv(envKey)
	}
	args := make([]string, argsLen)
	idx := 0
	for i, arg := range os.Args {
		if i == 0 {
			path, _ := osExec.LookPath(arg)
			arg = path
		}
		if arg != short && arg != long {
			args[idx] = arg
			idx++
		}

	}

	user, err := user.GetPwUID(uint32(os.Getuid()))
	if err != nil {
		sylog.Fatalf("failed to retrieve user information: %s", err)
	}

	// Append the user's real UID to the environment as _CONTAINERS_ROOTLESS_UID.
	// This is required in fakeroot builds that may use containers/image 5.7 and above.
	// https://github.com/containers/image/issues/1066
	// https://github.com/containers/image/blob/master/internal/rootless/rootless.go
	os.Setenv("_CONTAINERS_ROOTLESS_UID", strconv.Itoa(os.Getuid()))

	engineConfig := &fakerootConfig.EngineConfig{
		Args:     args,
		Envs:     os.Environ(),
		Home:     user.Dir,
		BuildEnv: true,
	}

	cfg := &config.Common{
		EngineName:   fakerootConfig.Name,
		ContainerID:  "fakeroot",
		EngineConfig: engineConfig,
	}

	err = starter.Exec(
		"Apptainer fakeroot",
		cfg,
		starter.UseSuid(useSuid),
	)
	sylog.Fatalf("%s", err)
}

func runBuild(cmd *cobra.Command, args []string) {
	if buildArgs.nvidia {
		os.Setenv("APPTAINER_NV", "1")
	}
	if buildArgs.nvccli {
		os.Setenv("APPTAINER_NVCCLI", "1")
	}
	if buildArgs.rocm {
		os.Setenv("APPTAINER_ROCM", "1")
	}
	if len(buildArgs.bindPaths) > 0 {
		os.Setenv("APPTAINER_BINDPATH", strings.Join(buildArgs.bindPaths, ","))
	}
	if len(buildArgs.mounts) > 0 {
		os.Setenv("APPTAINER_MOUNT", strings.Join(buildArgs.mounts, "\n"))
	}
	if buildArgs.writableTmpfs {
		if buildArgs.fakeroot {
			sylog.Fatalf("--writable-tmpfs option is not supported for fakeroot build")
		}
		os.Setenv("APPTAINER_WRITABLE_TMPFS", "1")
	}

	dest := args[0]
	spec := args[1]

	// check if target collides with existing file
	if err := checkBuildTarget(dest); err != nil {
		sylog.Fatalf("While checking build target: %s", err)
	}

	runBuildLocal(cmd.Context(), cmd, dest, spec)
	sylog.Infof("Build complete: %s", dest)
}

func runBuildLocal(ctx context.Context, cmd *cobra.Command, dst, spec string) {
	var keyInfo *cryptkey.KeyInfo
	if buildArgs.encrypt || promptForPassphrase || cmd.Flags().Lookup("pem-path").Changed {
		if os.Getuid() != 0 {
			sylog.Fatalf("You must be root to build an encrypted container")
		}

		k, err := getEncryptionMaterial(cmd)
		if err != nil {
			sylog.Fatalf("While handling encryption material: %v", err)
		}
		keyInfo = &k
	} else {
		_, passphraseEnvOK := os.LookupEnv("APPTAINER_ENCRYPTION_PASSPHRASE")
		_, pemPathEnvOK := os.LookupEnv("APPTAINER_ENCRYPTION_PEM_PATH")
		if passphraseEnvOK || pemPathEnvOK {
			sylog.Warningf("Encryption related env vars found, but --encrypt was not specified. NOT encrypting container.")
		}
	}

	imgCache := getCacheHandle(cache.Config{Disable: disableCache})
	if imgCache == nil {
		sylog.Fatalf("Failed to create an image cache handle")
	}

	if syscall.Getuid() != 0 && !buildArgs.fakeroot && fs.IsFile(spec) && !isImage(spec) {
		sylog.Fatalf("You must be the root user, however you can --fakeroot to build from an Apptainer recipe file")
	}

	err := checkSections()
	if err != nil {
		sylog.Fatalf("Could not check build sections: %v", err)
	}

	authConf, err := makeDockerCredentials(cmd)
	if err != nil {
		sylog.Fatalf("While creating Docker credentials: %v", err)
	}

	// parse definition to determine build source
	defs, err := build.MakeAllDefs(spec)
	if err != nil {
		sylog.Fatalf("Unable to build from %s: %v", spec, err)
	}

	hasLibrary := false

	// only resolve remote endpoints if library is a build source
	for _, d := range defs {
		if d.Header["bootstrap"] == "library" {
			hasLibrary = true
			break
		}
	}

	authToken := ""

	if hasLibrary {
		lc, err := getLibraryClientConfig(buildArgs.libraryURL)
		if err != nil {
			sylog.Fatalf("Unable to get library client configuration: %v", err)
		}
		buildArgs.libraryURL = lc.BaseURL
		authToken = lc.AuthToken
	}

	co, err := getKeyserverClientOpts(buildArgs.keyServerURL, endpoint.KeyserverVerifyOp)
	if err != nil {
		sylog.Fatalf("Unable to get key server client configuration: %v", err)
	}

	buildFormat := "sif"
	sandboxTarget := false
	if buildArgs.sandbox {
		buildFormat = "sandbox"
		sandboxTarget = true

	}

	b, err := build.New(
		defs,
		build.Config{
			Dest:      dst,
			Format:    buildFormat,
			NoCleanUp: buildArgs.noCleanUp,
			Opts: types.Options{
				ImgCache:          imgCache,
				TmpDir:            tmpDir,
				NoCache:           disableCache,
				Update:            buildArgs.update,
				Force:             forceOverwrite,
				Sections:          buildArgs.sections,
				NoTest:            buildArgs.noTest,
				NoHTTPS:           noHTTPS,
				LibraryURL:        buildArgs.libraryURL,
				LibraryAuthToken:  authToken,
				KeyServerOpts:     co,
				DockerAuthConfig:  authConf,
				EncryptionKeyInfo: keyInfo,
				FixPerms:          buildArgs.fixPerms,
				SandboxTarget:     sandboxTarget,
			},
		})
	if err != nil {
		sylog.Fatalf("Unable to create build: %v", err)
	}

	if err = b.Full(ctx); err != nil {
		sylog.Fatalf("While performing build: %v", err)
	}
}

func checkSections() error {
	var all, none bool
	for _, section := range buildArgs.sections {
		if section == "none" {
			none = true
		}
		if section == "all" {
			all = true
		}
	}

	if all && len(buildArgs.sections) > 1 {
		return fmt.Errorf("section specification error: cannot have all and any other option")
	}
	if none && len(buildArgs.sections) > 1 {
		return fmt.Errorf("section specification error: cannot have none and any other option")
	}

	return nil
}

func isImage(spec string) bool {
	i, err := image.Init(spec, false)
	if i != nil {
		_ = i.File.Close()
	}
	return err == nil
}

// getEncryptionMaterial handles the setting of encryption environment and flag parameters to eventually be
// passed to the crypt package for handling.
// This handles the APPTAINER_ENCRYPTION_PASSPHRASE/PEM_PATH envvars outside of cobra in order to
// enforce the unique flag/env precedence for the encryption flow
func getEncryptionMaterial(cmd *cobra.Command) (cryptkey.KeyInfo, error) {
	passphraseFlag := cmd.Flags().Lookup("passphrase")
	PEMFlag := cmd.Flags().Lookup("pem-path")
	passphraseEnv, passphraseEnvOK := os.LookupEnv("APPTAINER_ENCRYPTION_PASSPHRASE")
	pemPathEnv, pemPathEnvOK := os.LookupEnv("APPTAINER_ENCRYPTION_PEM_PATH")

	// checks for no flags/envvars being set
	if !(PEMFlag.Changed || pemPathEnvOK || passphraseFlag.Changed || passphraseEnvOK) {
		sylog.Fatalf("Unable to use container encryption. Must supply encryption material through environment variables or flags.")
	}

	// order of precedence:
	// 1. PEM flag
	// 2. Passphrase flag
	// 3. PEM envvar
	// 4. Passphrase envvar

	if PEMFlag.Changed {
		exists, err := fs.PathExists(encryptionPEMPath)
		if err != nil {
			sylog.Fatalf("Unable to verify existence of %s: %v", encryptionPEMPath, err)
		}

		if !exists {
			sylog.Fatalf("Specified PEM file %s: does not exist.", encryptionPEMPath)
		}

		sylog.Verbosef("Using pem path flag for encrypted container")

		// Check it's a valid PEM public key we can load, before starting the build (#4173)
		if cmd.Name() == "build" {
			if _, err := cryptkey.LoadPEMPublicKey(encryptionPEMPath); err != nil {
				sylog.Fatalf("Invalid encryption public key: %v", err)
			}
			// or a valid private key before launching the engine for actions on a container (#5221)
		} else {
			if _, err := cryptkey.LoadPEMPrivateKey(encryptionPEMPath); err != nil {
				sylog.Fatalf("Invalid encryption private key: %v", err)
			}
		}

		return cryptkey.KeyInfo{Format: cryptkey.PEM, Path: encryptionPEMPath}, nil
	}

	if passphraseFlag.Changed {
		sylog.Verbosef("Using interactive passphrase entry for encrypted container")
		passphrase, err := interactive.AskQuestionNoEcho("Enter encryption passphrase: ")
		if err != nil {
			return cryptkey.KeyInfo{}, err
		}
		if passphrase == "" {
			sylog.Fatalf("Cannot encrypt container with empty passphrase")
		}
		return cryptkey.KeyInfo{Format: cryptkey.Passphrase, Material: passphrase}, nil
	}

	if pemPathEnvOK {
		exists, err := fs.PathExists(pemPathEnv)
		if err != nil {
			sylog.Fatalf("Unable to verify existence of %s: %v", pemPathEnv, err)
		}

		if !exists {
			sylog.Fatalf("Specified PEM file %s: does not exist.", pemPathEnv)
		}

		sylog.Verbosef("Using pem path environment variable for encrypted container")
		return cryptkey.KeyInfo{Format: cryptkey.PEM, Path: pemPathEnv}, nil
	}

	if passphraseEnvOK {
		sylog.Verbosef("Using passphrase environment variable for encrypted container")
		return cryptkey.KeyInfo{Format: cryptkey.Passphrase, Material: passphraseEnv}, nil
	}

	return cryptkey.KeyInfo{}, nil
}
