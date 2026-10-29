// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

package image

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/pkg/inspect"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// OverlayBaseHashFile is the path, relative to the root of an overlay-only
// image built with `apptainer build --overlay`, of the file holding the
// hash of the base image it was built on top of.
const OverlayBaseHashFile = ".singularity.d/overlay-basehash"

// GetOverlayBaseHash returns the recorded base image hash for an overlay-only
// image. ok is false when the image has no recorded overlay base hash.
func GetOverlayBaseHash(path string) (hash string, ok bool, err error) {
	labels, err := GetOverlayBaseLabels(path)
	if err != nil {
		return "", false, err
	}
	hash, ok = labels["org.label-schema.usage.singularity.overlay.base-hash"]
	return strings.TrimSpace(hash), ok, nil
}

// GetOverlayBaseLabels returns the labels recorded in an overlay-only image.
func GetOverlayBaseLabels(path string) (map[string]string, error) {
	img, err := Init(path, false, false)
	if err != nil {
		return nil, err
	}
	defer img.File.Close()

	switch img.Type {
	case SANDBOX:
		labelsPath := filepath.Join(img.Path, ".singularity.d/labels.json")
		data, err := os.ReadFile(labelsPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		var labels map[string]string
		if err := json.Unmarshal(data, &labels); err != nil {
			sylog.Debugf("unable to parse %s: %v", data, err)
			return nil, nil
		}
		return labels, nil
	case SIF:
		r, err := NewSectionReader(img, SIFDescInspectMetadataJSON, -1)
		if errors.Is(err, ErrNoSection) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		metadata := new(inspect.Metadata)
		if err := json.NewDecoder(r).Decode(metadata); err != nil {
			sylog.Debugf("unable to decode SIF metadata: %v", err)
			return nil, nil
		}
		return metadata.Attributes.Labels, nil
	default:
		return nil, nil
	}
}
