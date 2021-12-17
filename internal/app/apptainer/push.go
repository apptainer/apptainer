// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	registryclient "github.com/apptainer/apptainer/internal/pkg/registry"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/sif/v2/pkg/sif"
	keyclient "github.com/sylabs/scs-key-client/client"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
	"golang.org/x/term"
)

// ErrRegistryUnsigned indicated that the image intended to be used is
// not signed, nor has an override for requiring a signature been provided
var ErrRegistryUnsigned = errors.New("image is not signed")

// RegistryPushSpec describes how a source image file should be pushed to a library server
type RegistryPushSpec struct {
	// SourceFile is the path to the container image to be pushed to the library
	SourceFile string
	// DestRef is the destination reference that the container image will be pushed to in the library
	DestRef string
	// Description is an optional string that describes the container image
	Description string
	// AllowUnsigned must be set to true to allow push of an unsigned container image to succeed
	AllowUnsigned bool
	// FrontendURI is the URI for the frontend (ie. https://cloud.sylabs.io)
	FrontendURI string
}

type progressCallback struct {
	progress *mpb.Progress
	bar      *mpb.Bar
	r        io.Reader
}

func (c *progressCallback) InitUpload(totalSize int64, r io.Reader) {
	// create bar
	c.progress = mpb.New()
	c.bar = c.progress.AddBar(totalSize,
		mpb.PrependDecorators(
			decor.Counters(decor.UnitKiB, "%.1f / %.1f"),
		),
		mpb.AppendDecorators(
			decor.Percentage(),
			decor.AverageSpeed(decor.UnitKiB, " % .1f "),
			decor.AverageETA(decor.ET_STYLE_GO),
		),
	)
	c.r = c.bar.ProxyReader(r)
}

func (c *progressCallback) GetReader() io.Reader {
	return c.r
}

func (c *progressCallback) Terminate() {
	c.bar.Abort(true)
}

func (c *progressCallback) Finish() {
	// wait for our bar to complete and flush
	c.progress.Wait()
}

// RegistryPush will upload an image file according to the provided RegistryPushSpec
// Before uploading, the image will be checked for a valid signature unless AllowUnsigned is true
func RegistryPush(ctx context.Context, pushSpec RegistryPushSpec, registryConfig *registryclient.Config, co []keyclient.Option) error {
	fi, err := os.Stat(pushSpec.SourceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("unable to open: %v: %v", pushSpec.SourceFile, err)
		}
		return err
	}

	arch, err := sifArch(pushSpec.SourceFile)
	if err != nil {
		return err
	}

	if !pushSpec.AllowUnsigned {
		// Check if the container has a valid signature.
		if err := Verify(ctx, pushSpec.SourceFile, OptVerifyUseKeyServer(co...)); err != nil {
			sylog.Warningf("%v", err)
			return ErrRegistryUnsigned
		}
	} else {
		sylog.Warningf("Skipping container verification")
	}

	registryClient, err := registryclient.NewClient(registryConfig)
	if err != nil {
		return fmt.Errorf("error initializing library client: %v", err)
	}

	// split library ref into components
	r, err := registryClient.Parse(pushSpec.DestRef)
	if err != nil {
		return fmt.Errorf("error parsing destination: %v", err)
	}

	// open image for uploading
	f, err := os.Open(pushSpec.SourceFile)
	if err != nil {
		return fmt.Errorf("error opening image %s for reading: %v", pushSpec.SourceFile, err)
	}
	defer f.Close()

	var progressBar registryclient.UploadCallback
	if !term.IsTerminal(2) {
		sylog.Infof("Uploading %d bytes\n", fi.Size())
	} else {
		progressBar = &progressCallback{}
	}

	var resp *registryclient.UploadImageComplete

	defer func(t time.Time) {
		if err == nil && resp != nil && progressBar == nil {
			sylog.Infof("Uploaded %d bytes in %v\n", fi.Size(), time.Since(t))
		}
	}(time.Now())

	if resp, err = registryClient.UploadImage(ctx, f, r.Host+r.Path, arch, r.Tags, pushSpec.Description, progressBar); err != nil {
		return err
	}

	// if the container already existed in the library, no upload was performed, so skip display
	if resp != nil {
		used, quota := resp.Quota.QuotaUsageBytes, resp.Quota.QuotaTotalBytes

		if quota == 0 {
			fmt.Printf("\nLibrary storage: using %s out of unlimited quota\n", fs.FindSize(used))
		} else {
			fmt.Printf("\nLibrary storage: using %s out of %s quota (%.1f%% used)\n", fs.FindSize(used), fs.FindSize(quota), float64(used)/float64(quota)*100.0)
		}

		fmt.Printf("Container URL: %s\n", pushSpec.FrontendURI+"/"+strings.TrimPrefix(resp.ContainerURL, "/"))
	}

	return nil
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
