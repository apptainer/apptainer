// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	"fmt"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	// emptyConfig is the OCI empty value (JSON)
	emptyConfig = "{}"
	// emptyConfigSize is the size of the empty config
	emptyConfigSize = 2
	// emptyConfigDigest is the sha256 digest of the empty value
	emptyConfigDigest = "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"

	// SifConfigMediaTypeV1 is the config descriptor mediaType for a SIF image.
	SifConfigMediaTypeV1 = "application/vnd.sylabs.sif.config.v1+json"
)

// SifImage implements a go-containerregistry v1.Image representing an ORAS / OCI artifact of a single SIF image.
type SifImage struct {
	manifest v1.Manifest
	layer    *SifLayer
}

var _ = v1.Image(&SifImage{})

// Layers returns the ordered collection of filesystem layers that comprise this image.
// The order of the list is oldest/base layer first, and most-recent/top layer last.
func (si *SifImage) Layers() ([]v1.Layer, error) {
	return []v1.Layer{si.layer}, nil
}

// MediaType of this image's manifest.
func (si *SifImage) MediaType() (types.MediaType, error) {
	return si.manifest.MediaType, nil
}

// Size returns the size of the manifest.
func (si *SifImage) Size() (int64, error) {
	return 0, nil
}

// ConfigName returns the hash of the image's config file, also known as
// the Image ID.
func (si *SifImage) ConfigName() (v1.Hash, error) {
	return si.manifest.Config.Digest, nil
}

// ConfigFile returns the hash of the image's config file, also known as
// the Image ID.
func (si *SifImage) ConfigFile() (*v1.ConfigFile, error) {
	return nil, nil
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (si *SifImage) RawConfigFile() ([]byte, error) {
	return []byte(emptyConfig), nil
}

// Digest returns the sha256 of this image's manifest.
func (si *SifImage) Digest() (v1.Hash, error) {
	return partial.Digest(si)
}

// Manifest returns this image's Manifest object.
func (si *SifImage) Manifest() (*v1.Manifest, error) {
	return &si.manifest, nil
}

// RawManifest returns the serialized bytes of Manifest()
func (si *SifImage) RawManifest() ([]byte, error) {
	return partial.RawManifest(si)
}

// LayerByDigest returns a Layer for interacting with a particular layer of
// the image, looking it up by "digest" (the compressed hash).
func (si *SifImage) LayerByDigest(hash v1.Hash) (v1.Layer, error) {
	if si.layer == nil || si.layer.hash != hash {
		return nil, fmt.Errorf("requested hash doesn't match SIF layer")
	}
	return si.layer, nil
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id"
// (the uncompressed hash).
func (si *SifImage) LayerByDiffID(hash v1.Hash) (v1.Layer, error) {
	return si.LayerByDigest(hash)
}

func NewImageFromSIF(file string, layerMediaType types.MediaType) (*SifImage, error) {
	si := SifImage{}

	sl, err := NewLayerFromSIF(file, layerMediaType)
	if err != nil {
		return nil, err
	}
	si.layer = sl

	lMediaType, err := si.layer.MediaType()
	if err != nil {
		return nil, err
	}
	lSize, err := si.layer.Size()
	if err != nil {
		return nil, err
	}
	lDigest, err := si.layer.Digest()
	if err != nil {
		return nil, err
	}

	emptyHash, err := v1.NewHash(emptyConfigDigest)
	if err != nil {
		return nil, err
	}

	//
	// Example manifest - config is always empty JSON '{}'. Single SIF file layer.
	//
	// {
	// 	"schemaVersion": 2,
	// 	"config": {
	// 	  "mediaType": "application/vnd.sylabs.sif.config.v1+json",
	// 	  "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
	// 	  "size": 2
	// 	},
	// 	"layers": [
	// 	  {
	// 		"mediaType": "application/vnd.sylabs.sif.layer.v1.sif",
	// 		"digest": "sha256:13e1552aaf6aa3916353730be52e06ec214ae8f8a89062cec1f33990b553a6c9",
	// 		"size": 29814784,
	// 		"annotations": {
	// 		  "org.opencontainers.image.title": "ubuntu_latest.sif"
	// 		}
	// 	  }
	// 	]
	//   }
	si.manifest = v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.MediaType(SifConfigMediaTypeV1),
			Digest:    emptyHash,
			Size:      emptyConfigSize,
		},
		Layers: []v1.Descriptor{
			{
				MediaType: lMediaType,
				Digest:    lDigest,
				Size:      lSize,
				Annotations: map[string]string{
					"org.opencontainers.image.title": filepath.Base(file),
				},
			},
		},
	}

	return &si, nil
}
