// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2024, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"

	"github.com/apptainer/apptainer/internal/pkg/client/shub"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// ShubConveyorPacker only needs to hold the conveyor to have the needed data to pack.
type ShubConveyorPacker struct {
	b *types.Bundle
	LocalPacker
}

// Get downloads container from Apptainerhub.
func (cp *ShubConveyorPacker) Get(ctx context.Context, b *types.Bundle) (err error) {
	sylog.Debugf("Getting container from Shub")

	cp.b = b

	src := `shub://` + b.Recipe.Header["from"]

	imagePath, err := shub.Pull(ctx, b.Opts.ImgCache, src, b.Opts.TmpDir, b.Opts.NoHTTPS)
	if err != nil {
		return fmt.Errorf("while fetching library image: %v", err)
	}

	// insert base metadata before unpacking fs
	if err = makeBaseEnv(cp.b.RootfsPath, true); err != nil {
		return fmt.Errorf("while inserting base environment: %v", err)
	}

	cp.LocalPacker, err = GetLocalPacker(ctx, imagePath, cp.b)

	return err
}

// CleanUp removes any tmpfs owned by the conveyorPacker on the filesystem
func (cp *ShubConveyorPacker) CleanUp() {
	cp.b.Remove()
}
