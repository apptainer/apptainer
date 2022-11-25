// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
	keyClient "github.com/apptainer/container-key-client/client"
	libClient "github.com/apptainer/container-library-client/client"
	"golang.org/x/term"
)

// ErrLibraryPullUnsigned indicates that the interactive portion of the pull was aborted.
var ErrLibraryPullUnsigned = errors.New("failed to verify container")

// pull will pull a library image into the cache if directTo="", or a specific file if directTo is set.
func pull(ctx context.Context, imgCache *cache.Handle, directTo string, imageRef *libClient.Ref, arch string, libraryConfig *libClient.Config) (string, error) {
	c, err := libClient.NewClient(libraryConfig)
	if err != nil {
		return "", fmt.Errorf("unable to initialize client library: %v", err)
	}

	ref := fmt.Sprintf("%s:%s", imageRef.Path, imageRef.Tags[0])

	libraryImage, err := c.GetImage(ctx, arch, ref)
	if err != nil {
		if errors.Is(err, libClient.ErrNotFound) {
			return "", fmt.Errorf("image does not exist in the library: %s (%s)", ref, arch)
		}
		return "", err
	}

	var progressBar libClient.ProgressBar
	if term.IsTerminal(2) {
		progressBar = &client.DownloadProgressBar{}
	}

	if directTo != "" {
		// Download direct to file
		if err := downloadWrapper(ctx, c, directTo, arch, imageRef, progressBar); err != nil {
			return "", fmt.Errorf("unable to download image: %v", err)
		}
		return directTo, nil
	}

	cacheEntry, err := imgCache.GetEntry(cache.LibraryCacheType, libraryImage.Hash)
	if err != nil {
		return "", fmt.Errorf("unable to check if %v exists in cache: %v", libraryImage.Hash, err)
	}
	defer cacheEntry.CleanTmp()

	if !cacheEntry.Exists {
		if err := downloadWrapper(ctx, c, cacheEntry.TmpPath, arch, imageRef, progressBar); err != nil {
			return "", fmt.Errorf("unable to download image: %v", err)
		}

		if cacheFileHash, err := libClient.ImageHash(cacheEntry.TmpPath); err != nil {
			return "", fmt.Errorf("error getting image hash: %v", err)
		} else if cacheFileHash != libraryImage.Hash {
			return "", fmt.Errorf("cached file hash(%s) and expected hash(%s) does not match", cacheFileHash, libraryImage.Hash)
		}

		if err := cacheEntry.Finalize(); err != nil {
			return "", err
		}
	} else {
		sylog.Infof("Using cached image")
	}

	return cacheEntry.Path, nil
}

// downloadWrapper calls DownloadImage() and outputs download summary if progressBar not specified.
func downloadWrapper(ctx context.Context, c *libClient.Client, imagePath, arch string, libraryRef *libClient.Ref, pb libClient.ProgressBar) error {
	sylog.Infof("Downloading library image")

	defer func(t time.Time) {
		if pb == nil {
			if fi, err := os.Stat(imagePath); err == nil {
				// Progress bar interface not specified; output summary to stdout
				sylog.Infof("Downloaded %d bytes in %v\n", fi.Size(), time.Since(t))
			}
		}
	}(time.Now())

	if err := DownloadImage(ctx, c, imagePath, arch, libraryRef, pb); err != nil {
		return err
	}
	return nil
}

// Pull will pull a library image to the cache or direct to a temporary file if cache is disabled
func Pull(ctx context.Context, imgCache *cache.Handle, pullFrom *libClient.Ref, arch string, tmpDir string, libraryConfig *libClient.Config) (imagePath string, err error) {
	directTo := ""

	if imgCache.IsDisabled() {
		file, err := os.CreateTemp(tmpDir, "sbuild-tmp-cache-")
		if err != nil {
			return "", fmt.Errorf("unable to create tmp file: %v", err)
		}
		directTo = file.Name()
		sylog.Infof("Downloading library image to tmp cache: %s", directTo)
	}

	return pull(ctx, imgCache, directTo, pullFrom, arch, libraryConfig)
}

// PullToFile will pull a library image to the specified location, through the cache, or directly if cache is disabled
func PullToFile(ctx context.Context, imgCache *cache.Handle, pullTo string, pullFrom *libClient.Ref, arch string, tmpDir string, libraryConfig *libClient.Config, co []keyClient.Option) (imagePath string, err error) {
	directTo := ""
	if imgCache.IsDisabled() {
		directTo = pullTo
		sylog.Debugf("Cache disabled, pulling directly to: %s", directTo)
	}

	src, err := pull(ctx, imgCache, directTo, pullFrom, arch, libraryConfig)
	if err != nil {
		return "", fmt.Errorf("error fetching image: %v", err)
	}

	if directTo == "" {
		// mode is before umask if pullTo doesn't exist
		err = fs.CopyFileAtomic(src, pullTo, 0o777)
		if err != nil {
			return "", fmt.Errorf("error copying image out of cache: %v", err)
		}
	}

	if err := apptainer.Verify(ctx, pullTo, apptainer.OptVerifyWithPGP(co...)); err != nil {
		sylog.Warningf("%v", err)
		return pullTo, ErrLibraryPullUnsigned
	}

	return pullTo, nil
}
