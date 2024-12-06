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
	"os"
	"path/filepath"
	"runtime"

	"github.com/apptainer/apptainer/docs"
	build_oci "github.com/apptainer/apptainer/internal/pkg/build/oci"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/client/library"
	"github.com/apptainer/apptainer/internal/pkg/client/net"
	"github.com/apptainer/apptainer/internal/pkg/client/oci"
	"github.com/apptainer/apptainer/internal/pkg/client/oras"
	"github.com/apptainer/apptainer/internal/pkg/client/shub"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/internal/pkg/util/uri"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

const (
	// LibraryProtocol holds the library base URI.
	LibraryProtocol = "library"
	// ShubProtocol holds the singularity hub base URI.
	ShubProtocol = "shub"
	// HTTPProtocol holds the remote http base URI.
	HTTPProtocol = "http"
	// HTTPSProtocol holds the remote https base URI.
	HTTPSProtocol = "https"
	// OrasProtocol holds the oras URI.
	OrasProtocol = "oras"
)

var (
	// pullLibraryURI holds the base URI to a Sylabs library API instance.
	pullLibraryURI string
	// pullImageName holds the name to be given to the pulled image.
	pullImageName string
	// unauthenticatedPull when true; won't ask to keep a unsigned container after pulling it.
	unauthenticatedPull bool
	// pullDir is the path that the containers will be pulled to, if set.
	pullDir string
	// pullArch is the architecture for which containers will be pulled from the
	// SCS library.
	pullArch string
	// pullArchVariant is the architecture variant, e.g., arm32v5, arm32v6, arm32v7, v5,v6,v7 are variants
	pullArchVariant string
	// pullSandbox indicates whether pulling images as sandbox format
	pullSandbox bool
)

// --arch
var pullArchFlag = cmdline.Flag{
	ID:           "pullArchFlag",
	Value:        &pullArch,
	DefaultValue: runtime.GOARCH,
	Name:         "arch",
	Usage:        "architecture to pull from library",
	EnvKeys:      []string{"PULL_ARCH"},
}

// --arch
var pullArchVariantFlag = cmdline.Flag{
	ID:           "pullArchVariantFlag",
	Value:        &pullArchVariant,
	DefaultValue: "",
	Name:         "arch-variant",
	Usage:        "architecture variant to pull from library",
	EnvKeys:      []string{"PULL_ARCH_VARIANT"},
}

// --library
var pullLibraryURIFlag = cmdline.Flag{
	ID:           "pullLibraryURIFlag",
	Value:        &pullLibraryURI,
	DefaultValue: "",
	Name:         "library",
	Usage:        "download images from the provided library",
	EnvKeys:      []string{"LIBRARY"},
}

// --name
var pullNameFlag = cmdline.Flag{
	ID:           "pullNameFlag",
	Value:        &pullImageName,
	DefaultValue: "",
	Name:         "name",
	Hidden:       true,
	Usage:        "specify a custom image name",
	EnvKeys:      []string{"PULL_NAME"},
}

// --dir
var pullDirFlag = cmdline.Flag{
	ID:           "pullDirFlag",
	Value:        &pullDir,
	DefaultValue: "",
	Name:         "dir",
	Usage:        "download images to the specific directory",
	EnvKeys:      []string{"PULLDIR", "PULLFOLDER"},
}

// --disable-cache
var pullDisableCacheFlag = cmdline.Flag{
	ID:           "pullDisableCacheFlag",
	Value:        &disableCache,
	DefaultValue: false,
	Name:         "disable-cache",
	Usage:        "do not use or create cached images/blobs",
	EnvKeys:      []string{"DISABLE_CACHE"},
}

// -U|--allow-unsigned
var pullAllowUnsignedFlag = cmdline.Flag{
	ID:           "pullAllowUnauthenticatedFlag",
	Value:        &unauthenticatedPull,
	DefaultValue: false,
	Name:         "allow-unsigned",
	ShortHand:    "U",
	Usage:        "do not require a signed container",
	EnvKeys:      []string{"ALLOW_UNSIGNED"},
	Deprecated:   `pull no longer exits with an error code in case of unsigned image. Now the flag only suppress warning message.`,
}

// --allow-unauthenticated
var pullAllowUnauthenticatedFlag = cmdline.Flag{
	ID:           "pullAllowUnauthenticatedFlag",
	Value:        &unauthenticatedPull,
	DefaultValue: false,
	Name:         "allow-unauthenticated",
	ShortHand:    "",
	Usage:        "do not require a signed container",
	EnvKeys:      []string{"ALLOW_UNAUTHENTICATED"},
	Hidden:       true,
}

// -s|--sandbox
var pullSandboxFlag = cmdline.Flag{
	ID:           "pullSandboxFlag",
	Value:        &pullSandbox,
	DefaultValue: false,
	Name:         "sandbox",
	ShortHand:    "",
	Usage:        "pull image as sandbox format (chroot directory structure)",
	EnvKeys:      []string{"SANDBOX"},
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(PullCmd)

		cmdManager.RegisterFlagForCmd(&commonForceFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullLibraryURIFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullNameFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&commonNoHTTPSFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&commonTmpDirFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullDisableCacheFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullDirFlag, PullCmd)

		cmdManager.RegisterFlagForCmd(&dockerHostFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&dockerUsernameFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&dockerPasswordFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&dockerLoginFlag, PullCmd)

		cmdManager.RegisterFlagForCmd(&buildNoCleanupFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullAllowUnsignedFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullAllowUnauthenticatedFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullArchFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&pullArchVariantFlag, PullCmd)
		cmdManager.RegisterFlagForCmd(&commonAuthFileFlag, PullCmd)

		cmdManager.RegisterFlagForCmd(&pullSandboxFlag, PullCmd)
	})
}

// PullCmd apptainer pull
var PullCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.RangeArgs(1, 2),
	Run:                   pullRun,
	Use:                   docs.PullUse,
	Short:                 docs.PullShort,
	Long:                  docs.PullLong,
	Example:               docs.PullExample,
}

func pullRun(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	imgCache := getCacheHandle(cache.Config{Disable: disableCache})
	if imgCache == nil {
		sylog.Fatalf("Failed to create an image cache handle")
	}

	pullFrom := args[len(args)-1]
	transport, ref := uri.Split(pullFrom)
	if ref == "" {
		sylog.Fatalf("Bad URI %s", pullFrom)
	}

	pullTo := pullImageName
	if pullTo == "" {
		pullTo = args[0]
		if len(args) == 1 {
			if transport == "" {
				sylog.Fatalf("No transport type URI supplied")
			}
			pullTo = uri.GetName(pullFrom) // TODO: If not library/shub & no name specified, simply put to cache
		}
	}

	if pullDir != "" {
		pullTo = filepath.Join(pullDir, pullTo)
	}

	_, err := os.Stat(pullTo)
	if !os.IsNotExist(err) {
		// image already exists
		if !forceOverwrite {
			sylog.Fatalf("Image file already exists: %q - will not overwrite", pullTo)
		}
	}

	switch transport {
	case LibraryProtocol:
		ref, err := library.NormalizeLibraryRef(pullFrom)
		if err != nil {
			sylog.Fatalf("Malformed library reference: %v", err)
		}

		if pullLibraryURI != "" && ref.Host != "" {
			sylog.Fatalf("Conflicting arguments; do not use --library with a library URI containing host name")
		}

		var libraryURI string
		if pullLibraryURI != "" {
			libraryURI = pullLibraryURI
		} else if ref.Host != "" {
			// override libraryURI if ref contains host name
			if noHTTPS {
				libraryURI = "http://" + ref.Host
			} else {
				libraryURI = "https://" + ref.Host
			}
		}

		lc, err := getLibraryClientConfig(libraryURI)
		if err != nil {
			sylog.Fatalf("Unable to get library client configuration: %v", err)
		}
		co, err := getKeyserverClientOpts("", endpoint.KeyserverVerifyOp)
		if err != nil {
			sylog.Fatalf("Unable to get keyserver client configuration: %v", err)
		}

		pullOpts := library.PullOptions{
			KeyClientOpts: co,
			LibraryConfig: lc,
		}

		_, err = library.PullToFile(ctx, imgCache, pullTo, ref, pullArch, tmpDir, pullOpts, pullSandbox)
		if err != nil && err != library.ErrLibraryPullUnsigned {
			sylog.Fatalf("While pulling library image: %v", err)
		}
		if err == library.ErrLibraryPullUnsigned {
			sylog.Warningf("Skipping container verification")
		}
	case ShubProtocol:
		_, err := shub.PullToFile(ctx, imgCache, pullTo, pullFrom, noHTTPS, pullSandbox)
		if err != nil {
			sylog.Fatalf("While pulling shub image: %v\n", err)
		}
	case OrasProtocol:
		ociAuth, err := makeOCICredentials(cmd)
		if err != nil {
			sylog.Fatalf("Unable to make docker oci credentials: %s", err)
		}

		_, err = oras.PullToFile(ctx, imgCache, pullTo, pullFrom, ociAuth, noHTTPS, reqAuthFile, pullSandbox)
		if err != nil {
			sylog.Fatalf("While pulling image from oci registry: %v", err)
		}
	case HTTPProtocol, HTTPSProtocol:
		_, err := net.PullToFile(ctx, imgCache, pullTo, pullFrom, pullSandbox)
		if err != nil {
			sylog.Fatalf("While pulling from image from http(s): %v\n", err)
		}
	case ociimage.SupportedTransport(transport):
		ociAuth, err := makeOCICredentials(cmd)
		if err != nil {
			sylog.Fatalf("While creating Docker credentials: %v", err)
		}

		arch, err := build_oci.ConvertArch(pullArch, pullArchVariant)
		if err != nil {
			sylog.Fatalf("While processing the arch and arch variant: %v", err)
			return
		}
		pullOpts := oci.PullOptions{
			TmpDir:      tmpDir,
			OciAuth:     ociAuth,
			DockerHost:  dockerHost,
			NoHTTPS:     noHTTPS,
			NoCleanUp:   buildArgs.noCleanUp,
			Pullarch:    arch,
			ReqAuthFile: reqAuthFile,
			Platform:    getOCIPlatform(),
		}

		_, err = oci.PullToFile(ctx, imgCache, pullTo, pullFrom, pullSandbox, pullOpts)
		if err != nil {
			sylog.Fatalf("While making image from oci registry: %v", err)
		}
	case "":
		sylog.Fatalf("No transport type URI supplied")
	default:
		sylog.Fatalf("Unsupported transport type: %s", transport)
	}
}
