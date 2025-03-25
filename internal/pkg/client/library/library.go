// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package library

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	libClient "github.com/apptainer/container-library-client/client"
	"github.com/ccoveille/go-safecast"
)

const defaultTag = "latest"

// Default empty description set by older pushes, which is not informative to display
// Comparison will be lower case as the GUI / code has used different capitalisation through time.
const noDescription = "no description"

func splitHostNameAndPath(ref string) (string, string) {
	ref = strings.TrimPrefix(ref, "library://")

	if strings.Count(ref, "/") <= 2 {
		// handle "library://container[:tag]", "library://collection/container[:tag]", or "library://entity/collection/container[:tag]"
		return "", ref
	}

	// handle "library://hostname/entity/collection/container[:tag]"
	c := strings.SplitN(ref, "/", 2)
	return c[0], c[1]
}

// NormalizeLibraryRef parses library ref and sets default tag, if necessary.
func NormalizeLibraryRef(ref string) (*libClient.Ref, error) {
	host, pathref := splitHostNameAndPath(ref)

	elem := strings.SplitN(pathref, ":", 2)

	var tags []string
	if len(elem) == 2 {
		tags = strings.Split(elem[1], ",")
	} else {
		tags = []string{defaultTag}
	}

	return &libClient.Ref{Host: host, Path: elem[0], Tags: tags}, nil
}

func getEnvInt64(key string, defval uint) (int64, error) {
	envKey := env.TrimApptainerKey(key)
	if env := env.GetenvLegacy(envKey, envKey); env != "" {
		if n, err := strconv.ParseInt(env, 10, 0); err == nil {
			return n, nil
		}
		sylog.Warningf("Error parsing %s; using default (%d)", key, defval)
	}
	return safecast.ToInt64(defval)
}

func getDownloadConfig() (libClient.Downloader, error) {
	// get downloader parameters from config
	conf := apptainerconf.GetCurrentConfig()
	if conf == nil {
		if strings.HasSuffix(os.Args[0], ".test") {
			// read config if doing unit tests
			var err error
			conf, err = apptainerconf.Parse(buildcfg.APPTAINER_CONF_FILE)
			if err != nil {
				sylog.Fatalf("unable to parse apptainer.conf file: %s", err)
			}
		} else {
			sylog.Fatalf("configuration not pre-loaded in getDownloadConfig")
		}
	}

	concurrency, err := getEnvInt64("APPTAINER_DOWNLOAD_CONCURRENCY", conf.DownloadConcurrency)
	if err != nil {
		return libClient.Downloader{}, fmt.Errorf("invalid download concurrency value (%v)", concurrency)
	}
	partSize, err := getEnvInt64("APPTAINER_DOWNLOAD_PART_SIZE", conf.DownloadPartSize)
	if err != nil {
		return libClient.Downloader{}, fmt.Errorf("invalid concurrent download part size (%v)", partSize)
	}
	bufferSize, err := getEnvInt64("APPTAINER_DOWNLOAD_BUFFER_SIZE", conf.DownloadBufferSize)
	if err != nil {
		return libClient.Downloader{}, fmt.Errorf("invalid concurrent download buffer size (%v)", bufferSize)
	}

	return libClient.Downloader{
		Concurrency: uint(concurrency),
		PartSize:    partSize,
		BufferSize:  bufferSize,
	}, nil
}

// DownloadImage is a helper function to wrap library image download operation
func DownloadImage(ctx context.Context, c *libClient.Client, imagePath, arch string, libraryRef *libClient.Ref, pb libClient.ProgressBar) error {
	// open destination file for writing
	f, err := os.OpenFile(imagePath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o777)
	if err != nil {
		return fmt.Errorf("error opening file %s for writing: %v", imagePath, err)
	}
	defer f.Close()

	var tag string
	if len(libraryRef.Tags) > 0 {
		tag = libraryRef.Tags[0]
	}

	spec, err := getDownloadConfig()
	if err != nil {
		return err
	}

	// call library client to download image
	if err := c.ConcurrentDownloadImage(ctx, f, arch, libraryRef.Path, tag, &spec, pb); err != nil {
		// Delete incomplete image file in the event of failure
		// we get here e.g. if the context is canceled by Ctrl-C
		sylog.Debugf("Cleaning up incomplete download: %s", imagePath)
		if err := os.Remove(imagePath); err != nil {
			sylog.Errorf("Error while removing incomplete download: %v", err)
		}

		return fmt.Errorf("error downloading image: %v", err)
	}

	return nil
}

// DownloadImageNoProgress downloads an image from the library without
// displaying a progress bar while doing so
func DownloadImageNoProgress(ctx context.Context, c *libClient.Client, imagePath, arch string, libraryRef *libClient.Ref) error {
	return DownloadImage(ctx, c, imagePath, arch, libraryRef, nil)
}

// SearchLibrary searches the library and outputs results to stdout
func SearchLibrary(ctx context.Context, c *libClient.Client, value, arch string, signed bool) error {
	if len(value) < 3 {
		return fmt.Errorf("bad query '%s'. You must search for at least 3 characters", value)
	}

	searchSpec := map[string]string{
		"value": value,
		"arch":  arch,
	}

	if signed {
		searchSpec["signed"] = "true"
	}

	results, err := c.Search(ctx, searchSpec)
	if err != nil {
		return err
	}

	numImages := len(results.Images)

	if numImages > 0 {
		imageList := []string{}
		for _, img := range results.Images {
			tagSpec := img.Hash
			if len(img.Tags) > 0 {
				tagSpec = strings.Join(img.Tags, ",")
			}
			imageItem := fmt.Sprintf("\tlibrary://%s/%s/%s:%s", img.EntityName, img.CollectionName, img.ContainerName, tagSpec)
			if img.Description != "" && strings.ToLower(img.Description) != noDescription {
				imageItem = imageItem + fmt.Sprintf("\n\t\t%s", img.Description)
			}
			if len(img.Fingerprints) > 0 {
				imageItem = imageItem + fmt.Sprintf("\n\t\tSigned by: %s", strings.Join(img.Fingerprints, ","))
			}
			imageList = append(imageList, imageItem)
		}
		sort.Strings(imageList)
		fmt.Printf("Found %d container images for %s matching %q:\n\n", numImages, arch, value)
		fmt.Println(strings.Join(imageList, "\n\n"))
		fmt.Printf("\n")
	} else {
		fmt.Printf("No container images found for %s matching %q.\n\n", arch, value)
	}

	return nil
}
