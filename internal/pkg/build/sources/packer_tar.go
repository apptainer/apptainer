// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// TarPacker handles packing from a tar file for data partition builds
type TarPacker struct {
	srcfile string
	b       *types.Bundle
}

// Pack converts a tar file to a squashfs image using mksquashfs -tar
// and sets b.RootfsImage to the resulting squashfs path
func (tp *TarPacker) Pack(ctx context.Context) (*types.Bundle, error) {
	sylog.Debugf("Packing from Tar")

	f, err := os.CreateTemp(tp.b.TmpDir, "tar-squashfs-")
	if err != nil {
		return nil, fmt.Errorf("while creating temporary file for squashfs: %v", err)
	}
	fsPath := f.Name()
	f.Close()

	mksquashfsArgs := []string{"-noappend", "-all-root"}

	if tp.b.Opts.MksquashfsArgs != "" {
		extraArgs := append(mksquashfsArgs, strings.Fields(tp.b.Opts.MksquashfsArgs)...)
		mksquashfsArgs = extraArgs
	}

	tarFile, err := os.Open(tp.srcfile)
	if err != nil {
		return nil, fmt.Errorf("could not open tar file %s: %v", tp.srcfile, err)
	}
	defer tarFile.Close()

	mksquashfsPath, err := bin.FindBin("mksquashfs")
	if err != nil {
		return nil, fmt.Errorf("could not create squashfs, mksquashfs not found: %v", err)
	}

	mksquashfsArgs = append([]string{"-", fsPath, "-tar"}, mksquashfsArgs...)
	cmd := exec.CommandContext(ctx, mksquashfsPath, mksquashfsArgs...)
	cmd.Stdin = tarFile

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mksquashfs command failed: %v: %s", err, output)
	}

	tp.b.RootfsImage = fsPath
	return tp.b, nil
}
