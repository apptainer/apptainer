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
	"errors"
	"io"

	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
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
