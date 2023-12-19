// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// This file is to add progress bar support for oras protocol.
package oras

import (
	"io"

	"github.com/apptainer/apptainer/internal/pkg/client"
	libClient "github.com/apptainer/container-library-client/client"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var pb *progressBar

type progressBar struct {
	libClient.ProgressBar
	previous int64
}

func showProgressBar(updates chan v1.Update) error {
	for update := range updates {
		if update.Error != nil {
			if pb != nil {
				pb.Abort(true)
			}
			return update.Error
		}

		if update.Complete == update.Total {
			break
		}

		if pb == nil {
			pb = &progressBar{
				ProgressBar: &client.DownloadProgressBar{},
				previous:    0,
			}
			pb.Init(update.Total)
		}
		pb.IncrBy(int(update.Complete - pb.previous))
		pb.previous = update.Complete
	}

	return nil
}

type writerWithProgressBar struct {
	io.Writer
	libClient.ProgressBar
}

func (w *writerWithProgressBar) Write(p []byte) (n int, err error) {
	n, err = w.Writer.Write(p)
	if err != nil {
		w.Abort(true)
		return n, err
	}
	w.IncrBy(n)
	return n, err
}
