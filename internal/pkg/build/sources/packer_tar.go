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
	"path/filepath"
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

// getDecompressCmd returns the decompression command and arguments for the given file extension.
// For compressed tar files, it uses the native compression tools (gzip, xz, zstd, lz4, lzop)
// directly with -dc flags instead of the wrapper programs (zcat, xzcat, zstdcat, lz4cat).
func getDecompressCmd(filename string) (string, []string, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".tar", "":
		return "cat", []string{}, nil
	case ".gz", ".tgz":
		return "gzip", []string{"-dc"}, nil
	case ".xz", ".txz":
		return "xz", []string{"-dc"}, nil
	case ".zst":
		return "zstd", []string{"-dc"}, nil
	case ".lz4":
		return "lz4", []string{"-dc"}, nil
	case ".lzo":
		return "lzop", []string{"-dc"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported tar extension: %s", ext)
	}
}

// Pack converts a tar file to a squashfs image using mksquashfs -tar
// and sets b.RootfsImage to the resulting squashfs path
func (tp *TarPacker) Pack(ctx context.Context) (*types.Bundle, error) {
	sylog.Debugf("Packing from Tar")

	decompressCmd, decompressArgs, err := getDecompressCmd(tp.srcfile)
	if err != nil {
		return nil, err
	}

	sylog.Debugf("Using decompress command: %s %v", decompressCmd, decompressArgs)

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

	mksquashfsPath, err := bin.FindBin("mksquashfs")
	if err != nil {
		return nil, fmt.Errorf("could not create squashfs, mksquashfs not found: %v", err)
	}

	if decompressCmd != "cat" {
		decompressCmdPath, err := exec.LookPath(decompressCmd)
		if err != nil {
			return nil, fmt.Errorf("could not find decompression command: %v", err)
		}
		decompressCmdArgs := append(decompressArgs, tp.srcfile)
		decompressCmdObj := exec.CommandContext(ctx, decompressCmdPath, decompressCmdArgs...)
		decompressPipe, err := decompressCmdObj.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("could not create pipe for decompress: %v", err)
		}
		if err := decompressCmdObj.Start(); err != nil {
			return nil, fmt.Errorf("could not start decompress command: %v", err)
		}
		mksquashfsCmdArgs := append([]string{"-", fsPath, "-tar"}, mksquashfsArgs...)
		mksquashfsCmdObj := exec.CommandContext(ctx, mksquashfsPath, mksquashfsCmdArgs...)
		mksquashfsCmdObj.Stdin = decompressPipe
		output, err := mksquashfsCmdObj.CombinedOutput()
		if err != nil {
			decompressCmdObj.Wait()
			return nil, fmt.Errorf("mksquashfs command failed: %v: %s", err, output)
		}
		decompressCmdObj.Wait()
	} else {
		tarFile, err := os.Open(tp.srcfile)
		if err != nil {
			return nil, fmt.Errorf("could not open tar file %s: %v", tp.srcfile, err)
		}
		defer tarFile.Close()

		mksquashfsArgs = append([]string{"-", fsPath, "-tar"}, mksquashfsArgs...)
		cmd := exec.CommandContext(ctx, mksquashfsPath, mksquashfsArgs...)
		cmd.Stdin = tarFile

		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("mksquashfs command failed: %v: %s", err, output)
		}
	}

	tp.b.RootfsImage = fsPath
	return tp.b, nil
}
