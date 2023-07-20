// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
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

	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/pkg/sylog"
	scslibrary "github.com/apptainer/container-library-client/client"
	"github.com/apptainer/sif/v2/pkg/sif"
	"golang.org/x/term"
)

// Push will upload an image file to the library.
// Returns the upload completion response on success, containing container path and quota usage.
func Push(ctx context.Context, sourceFile string, destRef *scslibrary.Ref, desc string, libraryConfig *scslibrary.Config) (uploadResponse *scslibrary.UploadImageComplete, err error) {
	fi, err := os.Stat(sourceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to open: %v: %v", sourceFile, err)
		}
		return nil, err
	}

	arch, err := sifArch(sourceFile)
	if err != nil {
		return nil, err
	}

	libraryClient, err := scslibrary.NewClient(libraryConfig)
	if err != nil {
		return nil, fmt.Errorf("error initializing library client: %v", err)
	}

	if destRef.Host != "" && destRef.Host != libraryClient.BaseURL.Host {
		return nil, errors.New("push to location other than current remote is not supported")
	}

	// open image for uploading
	f, err := os.Open(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("error opening image %s for reading: %v", sourceFile, err)
	}
	defer f.Close()

	var progressBar scslibrary.UploadCallback
	if term.IsTerminal(2) {
		progressBar = &client.UploadProgressBar{}
	}

	resp, err := libraryClient.UploadImage(ctx, f, destRef.Path, arch, destRef.Tags, desc, progressBar)
	defer func(t time.Time) {
		if err == nil && resp != nil && progressBar == nil {
			sylog.Infof("Uploaded %d bytes in %v\n", fi.Size(), time.Since(t))
		}
	}(time.Now())

	return resp, err
}

func sifArch(filename string) (string, error) {
	f, err := sif.LoadContainerFromPath(filename, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return "", fmt.Errorf("unable to open: %v: %w", filename, err)
	}
	defer f.UnloadContainer()

	arch := f.PrimaryArch()
	if arch == "unknown" {
		return arch, fmt.Errorf("unknown architecture in SIF file")
	}
	return arch, nil
}
