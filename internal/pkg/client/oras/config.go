// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	// UnknownConfigMediaTypeV1 is the mediaType for the "config" that is always empty ({}).
	UnknownConfigMediaTypeV1 = "application/vnd.unknown.config.v1+json"

	// SifConfigMediaTypeV1 is the config descriptor mediaType for a SIF image.
	SifConfigMediaTypeV1 = "application/vnd.sylabs.sif.config.v1+json"

	// emptyConfig is the OCI empty value (JSON)
	emptyConfig = "{}"
	// emptyConfigSize is the size of the empty config
	emptyConfigSize = 2
	// emptyConfigDigest is the sha256 digest of the empty value
	emptyConfigDigest = "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
)

// SifConfig implements a go-containerregistry v1.Descriptor representing an ORAS / OCI artifact.
type SifConfig struct {
	mediaType types.MediaType
}

func (sc *SifConfig) Digest() (v1.Hash, error) {
	return v1.NewHash(emptyConfigDigest)
}

func (sc *SifConfig) Size() (int64, error) {
	return emptyConfigSize, nil
}

func (sc *SifConfig) Data() ([]byte, error) {
	return []byte(emptyConfig), nil
}

func (sc *SifConfig) MediaType() (types.MediaType, error) {
	return sc.mediaType, nil
}

// NewConfigFromSIF creates a new config, with mt as the MediaType.
// The MediaType should always be set SifConfigMediaTypeV1 in production.
func NewConfigFromSIF(_ string, mt types.MediaType) (*SifConfig, error) {
	sc := SifConfig{
		mediaType: mt,
	}

	return &sc, nil
}
