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
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/ccoveille/go-safecast"
)

const (
	squashfsMagic    = "\x68\x73\x71\x73"
	squashfsZlib     = 1
	squashfsLzmaComp = 2
	squashfsLzoComp  = 3
	squashfsXzComp   = 4
	squashfsLz4Comp  = 5
	squashfsZstdComp = 6
)

// this represents the superblock of a v4 squashfs image
// previous versions of the superblock contain the major and minor versions
// at the same location so we can use this struct to deduce the version
// of the image
type squashfsInfo struct {
	Magic       [4]byte
	Inodes      uint32
	MkfsTime    uint32
	BlockSize   uint32
	Fragments   uint32
	Compression uint16
	BlockLog    uint16
	Flags       uint16
	NoIDs       uint16
	Major       uint16
	Minor       uint16
}

type squashfsFormat struct{}

// parseSquashfsHeader de-serialized the squashfs super block from the supplied byte array
// return a struct describing a v4 superblock, the offset of where the superblock began
func parseSquashfsHeader(b []byte) (*squashfsInfo, uint64, error) {
	var offset uint64

	launchStart := bytes.Index(b, []byte(launchString))
	if launchStart > 0 {
		launchEnd, err := safecast.Convert[uint64](launchStart + len(launchString) + 1)
		if err != nil {
			return nil, 0, err
		}
		offset += launchEnd
	}
	sinfo := &squashfsInfo{}

	if uintptr(offset)+unsafe.Sizeof(sinfo) >= uintptr(len(b)) {
		return nil, offset, fmt.Errorf("can't find squashfs information header")
	}

	buffer := bytes.NewReader(b[offset:])

	if err := binary.Read(buffer, binary.LittleEndian, sinfo); err != nil {
		return nil, offset, fmt.Errorf("can't read the top of the image")
	}
	if !bytes.Equal(sinfo.Magic[:], []byte(squashfsMagic)) {
		return nil, offset, fmt.Errorf("not a valid squashfs image")
	}

	return sinfo, offset, nil
}

// CheckSquashfsHeader checks if byte content contains a valid squashfs header
// and returns offset where squashfs partition starts
func CheckSquashfsHeader(b []byte) (uint64, error) {
	sinfo, offset, err := parseSquashfsHeader(b)
	if err != nil {
		return offset, debugErrorf("while parsing squashfs super block: %v", err)
	}

	if sinfo.Compression != squashfsZlib {
		compressionType := ""
		switch sinfo.Compression {
		case squashfsLzmaComp:
			compressionType = "lzma"
		case squashfsLz4Comp:
			compressionType = "lz4"
		case squashfsLzoComp:
			compressionType = "lzo"
		case squashfsXzComp:
			compressionType = "xz"
		case squashfsZstdComp:
			compressionType = "zstd"
		default:
			return 0, fmt.Errorf("corrupted image: unknown compression algorithm value %d", sinfo.Compression)
		}
		sylog.Debugf("squashfs image compression type %s", compressionType)
	}
	return offset, nil
}

func (f *squashfsFormat) initializer(img *Image, fileinfo os.FileInfo) error {
	if fileinfo.IsDir() {
		return debugError("not a squashfs image")
	}
	b := make([]byte, bufferSize)
	if n, err := img.File.Read(b); err != nil || n != bufferSize {
		return debugErrorf("can't read first %d bytes: %v", bufferSize, err)
	}
	offset, err := CheckSquashfsHeader(b)
	if err != nil {
		return err
	}
	fSize, err := safecast.Convert[uint64](fileinfo.Size())
	if err != nil {
		return err
	}
	img.Type = SQUASHFS
	img.Partitions = []Section{
		{
			Offset:       offset,
			Size:         fSize - offset,
			ID:           1,
			Type:         SQUASHFS,
			Name:         RootFs,
			AllowedUsage: RootFsUsage | OverlayUsage | DataUsage,
		},
	}

	if img.Writable {
		// we set Writable to appropriate value to match the
		// image open mode as some code may want to ignore this
		// error by using IsReadOnlyFilesytem check
		img.Writable = false

		return &readOnlyFilesystemError{
			"could not set " + img.Path + " image writable: squashfs is a read-only filesystem",
		}
	}

	return nil
}

func (f *squashfsFormat) openMode(_ bool) int {
	return os.O_RDONLY
}

func (f *squashfsFormat) lock(_ *Image) error {
	return nil
}
