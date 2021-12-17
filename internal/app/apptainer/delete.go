// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"context"

	registryclient "github.com/apptainer/apptainer/internal/pkg/registry"
	"github.com/pkg/errors"
)

// DeleteImage deletes an image from a remote registry.
func DeleteImage(ctx context.Context, registryConfig *registryclient.Config, imageRef, arch string) error {
	registryClient, err := registryclient.NewClient(registryConfig)
	if err != nil {
		return errors.Wrap(err, "couldn't create a new client")
	}

	err = registryClient.DeleteImage(ctx, imageRef, arch)
	if err != nil {
		return errors.Wrap(err, "couldn't delete requested image")
	}

	return nil
}
