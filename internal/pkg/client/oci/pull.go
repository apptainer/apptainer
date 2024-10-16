// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/build"
	"github.com/apptainer/apptainer/internal/pkg/build/oci"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	buildtypes "github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type PullOptions struct {
	TmpDir      string
	OciAuth     *authn.AuthConfig
	DockerHost  string
	NoHTTPS     bool
	NoCleanUp   bool
	Pullarch    string
	ReqAuthFile string
}

// transportOptions maps PullOptions to OCI image transport options
func transportOptions(opts PullOptions) *ociimage.TransportOptions {
	return &ociimage.TransportOptions{
		AuthConfig:       opts.OciAuth,
		AuthFilePath:     ociauth.ChooseAuthFile(opts.ReqAuthFile),
		Insecure:         opts.NoHTTPS,
		TmpDir:           opts.TmpDir,
		UserAgent:        useragent.Value(),
		DockerDaemonHost: opts.DockerHost,
		Platform:         v1.Platform{},
	}
}

// pull will build a SIF image into the cache if directTo="", or a specific file if directTo is set.
func pull(ctx context.Context, imgCache *cache.Handle, directTo, pullFrom string, opts PullOptions) (imagePath string, err error) {
	// DockerInsecureSkipTLSVerify is set only if --no-https is specified to honor
	// configuration from /etc/containers/registries.conf because DockerInsecureSkipTLSVerify
	// can have three possible values true/false and undefined, so we left it as undefined instead
	// of forcing it to false in order to delegate decision to /etc/containers/registries.conf:
	// https://github.com/apptainer/singularity/issues/5172
	to := transportOptions(opts)
	if opts.Pullarch != "" {
		if arch, ok := oci.ArchMap[opts.Pullarch]; ok {
			to.Platform = v1.Platform{
				Architecture: arch.Arch,
				Variant:      arch.Var,
			}
		} else {
			keys := reflect.ValueOf(oci.ArchMap).MapKeys()
			return "", fmt.Errorf("failed to parse the arch value: %s, should be one of %v", opts.Pullarch, keys)
		}
	}
	hash, err := oci.ImageDigest(ctx, pullFrom, to)
	if err != nil {
		return "", fmt.Errorf("failed to get checksum for %s: %s", pullFrom, err)
	}

	if directTo != "" {
		sylog.Infof("Converting OCI blobs to SIF format")
		if err := convertOciToSIF(ctx, imgCache, pullFrom, directTo, opts); err != nil {
			return "", fmt.Errorf("while building SIF from layers: %v", err)
		}
		imagePath = directTo
	} else {

		cacheEntry, err := imgCache.GetEntry(cache.OciTempCacheType, hash)
		if err != nil {
			return "", fmt.Errorf("unable to check if %v exists in cache: %v", hash, err)
		}
		defer cacheEntry.CleanTmp()
		if !cacheEntry.Exists {
			sylog.Infof("Converting OCI blobs to SIF format")

			if err := convertOciToSIF(ctx, imgCache, pullFrom, cacheEntry.TmpPath, opts); err != nil {
				return "", fmt.Errorf("while building SIF from layers: %v", err)
			}

			err = cacheEntry.Finalize()
			if err != nil {
				return "", err
			}

		} else {
			sylog.Infof("Using cached SIF image")
		}
		imagePath = cacheEntry.Path
	}

	return imagePath, nil
}

// convertOciToSIF will convert an OCI source into a SIF using the build routines
func convertOciToSIF(ctx context.Context, imgCache *cache.Handle, image, cachedImgPath string, opts PullOptions) error {
	if imgCache == nil {
		return fmt.Errorf("image cache is undefined")
	}

	b, err := build.NewBuild(
		image,
		build.Config{
			Dest:      cachedImgPath,
			Format:    "sif",
			NoCleanUp: opts.NoCleanUp,
			Opts: buildtypes.Options{
				TmpDir:           opts.TmpDir,
				NoCache:          imgCache.IsDisabled(),
				NoTest:           true,
				NoHTTPS:          opts.NoHTTPS,
				OCIAuthConfig:    opts.OciAuth,
				DockerDaemonHost: opts.DockerHost,
				ImgCache:         imgCache,
				Arch:             opts.Pullarch,
				ReqAuthFile:      opts.ReqAuthFile,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create new build: %v", err)
	}

	return b.Full(ctx)
}

// Pull will build a SIF image to the cache or direct to a temporary file if cache is disabled
func Pull(ctx context.Context, imgCache *cache.Handle, pullFrom string, opts PullOptions) (imagePath string, err error) {
	directTo := ""

	if imgCache.IsDisabled() {
		file, err := os.CreateTemp(opts.TmpDir, "sbuild-tmp-cache-")
		if err != nil {
			return "", fmt.Errorf("unable to create tmp file: %v", err)
		}
		directTo = file.Name()
		sylog.Infof("Downloading library image to tmp cache: %s", directTo)
	}

	return pull(ctx, imgCache, directTo, pullFrom, opts)
}

// PullToFile will build a SIF image from the specified oci URI and place it at the specified dest
func PullToFile(ctx context.Context, imgCache *cache.Handle, pullTo, pullFrom string, sandbox bool, opts PullOptions) (imagePath string, err error) {
	directTo := ""
	if imgCache.IsDisabled() {
		directTo = pullTo
		sylog.Debugf("Cache disabled, pulling directly to: %s", directTo)
	}

	src, err := pull(ctx, imgCache, directTo, pullFrom, opts)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported image-specific operation on artifact with type \"application/vnd.unknown.config.v1+json\"") {
			return "", fmt.Errorf("%v; try changing the protocol to oras://", err)
		}
		return "", fmt.Errorf("error fetching image to cache: %v", err)
	}

	if directTo == "" && !sandbox {
		// mode is before umask if pullTo doesn't exist
		err = fs.CopyFileAtomic(src, pullTo, 0o777)
		if err != nil {
			return "", fmt.Errorf("error copying image out of cache: %v", err)
		}
	}

	if sandbox {
		if err := client.ConvertSifToSandbox(directTo, src, pullTo); err != nil {
			return "", err
		}
	}

	return pullTo, nil
}
