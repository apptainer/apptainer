// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	// SifLayerMediaTypeV1 is the mediaType for the "layer" which contains the actual SIF file
	SifLayerMediaTypeV1 = "application/vnd.sylabs.sif.layer.v1.sif"

	// SifLayerMediaTypeProto is the mediaType from prototyping and Singularity
	// <3.7 which unfortunately includes a typo and doesn't have a version suffix
	// See: https://github.com/hpcng/singularity/issues/4437
	SifLayerMediaTypeProto = "appliciation/vnd.sylabs.sif.layer.tar"
)

// SifLayer implements a go-containerregistry v1.Layer backed by a SIF file, for
// ORAS / OCI artifact usage.
type SifLayer struct {
	size      int64
	rc        io.ReadCloser
	hash      v1.Hash
	mediaType types.MediaType
}

var _ = v1.Layer(&SifLayer{})

func (sl *SifLayer) Digest() (v1.Hash, error) {
	return sl.hash, nil
}

func (sl *SifLayer) DiffID() (v1.Hash, error) {
	return sl.hash, nil
}

func (sl *SifLayer) Compressed() (io.ReadCloser, error) {
	return sl.rc, nil
}

func (sl *SifLayer) Uncompressed() (io.ReadCloser, error) {
	return sl.rc, nil
}

func (sl *SifLayer) Size() (int64, error) {
	return sl.size, nil
}

func (sl *SifLayer) MediaType() (types.MediaType, error) {
	return sl.mediaType, nil
}

// NewLayerFromSIF creates a new layer, backed by file, with mt as the
// MediaType. The MediaType should always be set SifLayerMediaTypeV1 in
// production. It is configurable so that we can implement tests which use the
// old prototype media value.
func NewLayerFromSIF(file string, mt types.MediaType) (*SifLayer, error) {
	sl := SifLayer{
		mediaType: mt,
	}

	fi, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	sl.size = fi.Size()

	hash, err := ImageHash(file)
	if err != nil {
		return nil, err
	}
	sl.hash = hash

	rc, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	sl.rc = rc

	return &sl, nil
}
