// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"bytes"
	"fmt"
	"os"

	"github.com/ccoveille/go-safecast/v2"
)

const (
	tarBlockSize   = 512
	tarustarMagic  = "ustar"
	tarGnuMagic    = "GNU\000"
	tarNameOffset  = 0
	tarNameSize    = 100
	tarMagicOffset = 257
	tarMagicSize   = 6
)

type tarFormat struct{}

// CheckTarHeader checks if byte content contains a valid tar header
// and returns 0 (tar headers start at offset 0)
func CheckTarHeader(b []byte) error {
	if len(b) < tarBlockSize {
		return debugError("not enough data for tar header")
	}

	magic := b[tarMagicOffset : tarMagicOffset+tarMagicSize]

	if !bytes.Equal(magic[:5], []byte(tarustarMagic)) && !bytes.Equal(magic[:3], []byte(tarGnuMagic)) {
		return debugError("not a valid tar image")
	}

	return nil
}

func (f *tarFormat) initializer(img *Image, fileinfo os.FileInfo) error {
	if fileinfo.IsDir() {
		return debugError("not a tar image")
	}
	b := make([]byte, tarBlockSize)
	if n, err := img.File.Read(b); err != nil || n != tarBlockSize {
		return debugErrorf("can't read first %d bytes: %v", tarBlockSize, err)
	}
	if err := CheckTarHeader(b); err != nil {
		return err
	}
	fSize, err := safecast.Convert[uint64](fileinfo.Size())
	if err != nil {
		return err
	}
	img.Type = TAR
	img.Partitions = []Section{
		{
			Offset:       0,
			Size:         fSize,
			ID:           1,
			Type:         TAR,
			Name:         RootFs,
			AllowedUsage: DataUsage,
		},
	}

	return nil
}

func (f *tarFormat) openMode(writable bool) int {
	if writable {
		return os.O_RDWR
	}
	return os.O_RDONLY
}

func (f *tarFormat) lock(img *Image) error {
	if err := lockSection(img, img.Partitions[0]); err != nil {
		return fmt.Errorf("while locking tar partition from %s: %s", img.Path, err)
	}
	return nil
}
