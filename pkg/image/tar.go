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
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"

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

// checkTarHeaderBytes checks if tar header bytes contain a valid tar header
func checkTarHeaderBytes(b []byte) error {
	if len(b) < tarBlockSize {
		return debugError("not enough data for tar header")
	}

	magic := b[tarMagicOffset : tarMagicOffset+tarMagicSize]

	if !bytes.Equal(magic[:5], []byte(tarustarMagic)) && !bytes.Equal(magic[:3], []byte(tarGnuMagic)) {
		return debugError("not a valid tar image")
	}

	return nil
}

// hasGzipMagic returns true if the bytes have the gzip magic header (0x1f 0x8b)
func hasGzipMagic(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

// CheckTarHeader checks if byte content contains a valid tar header
// and returns 0 (tar headers start at offset 0)
func CheckTarHeader(b []byte) error {
	return checkTarHeaderBytes(b)
}

// CheckTarHeaderWithDecompression checks if byte content contains a valid tar header,
// or a compressed tar header. For gzip, it checks magic bytes and decompresses if needed.
// For other formats (xz, zst, lz4, lzo), it uses the file extension to call the appropriate
// decompression command (xz, zstd, lz4, lzop with -dc flags).
func CheckTarHeaderWithDecompression(b []byte, filename string) error {
	ext := filepath.Ext(filename)

	switch ext {
	case ".tar", "":
		// For uncompressed .tar, just check the tar header
		return CheckTarHeader(b)

	case ".gz", ".tgz":
		// For .gz/.tgz, check for gzip magic and decompress
		if hasGzipMagic(b) {
			gzipReader, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				return debugErrorf("failed to create gzip reader: %v", err)
			}
			defer gzipReader.Close()

			decompressedBytes := make([]byte, tarBlockSize)
			if n, err := gzipReader.Read(decompressedBytes); err != nil || n < tarBlockSize {
				return debugError("failed to read decompressed tar header")
			}

			return checkTarHeaderBytes(decompressedBytes)
		}

		return debugError("not a valid gzip compressed tar image")

	case ".xz", ".txz", ".zst", ".lz4", ".lzo":
		// For other compressed formats, just check extension
		// The actual decompression will happen later with external commands
		return nil

	default:
		return debugError("not a valid tar image")
	}
}

func (f *tarFormat) initializer(img *Image, fileinfo os.FileInfo, _ bool) error {
	if fileinfo.IsDir() {
		return debugError("not a tar image")
	}
	// Read the first tarBlockSize bytes (or all available if less)
	b := make([]byte, tarBlockSize)
	n, err := img.File.Read(b)
	if err != nil {
		return debugErrorf("can't read first %d bytes: %v", tarBlockSize, err)
	}
	if n == 0 {
		return debugError("can't read first byte")
	}
	if err := CheckTarHeaderWithDecompression(b[:n], img.Path); err != nil {
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
