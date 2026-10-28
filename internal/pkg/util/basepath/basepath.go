// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package basepath implements discovery of a base container image, given a
// hash and a list of paths to search, as used by the `--basepath` action
// option to locate the base image that an overlay-only image (built with
// `apptainer build --overlay`) was built on top of.
package basepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/pkg/image"
	"github.com/google/go-containerregistry/pkg/name"
)

const (
	overlayBootstrapLabel = "org.label-schema.usage.singularity.deffile.bootstrap"
	overlayFromLabel      = "org.label-schema.usage.singularity.deffile.from"
	overlayHashLabel      = "org.label-schema.usage.singularity.overlay.base-hash"
)

// isImage returns true if path directly refers to a SIF file, or a sandbox
// directory (i.e. one containing a ".singularity.d" subdirectory, as
// created by every apptainer build). A plain directory that does not look
// like a sandbox is treated as a directory to be searched for hash instead.
func isImage(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !info.IsDir() {
		img, err := image.Init(path, false, false)
		if err != nil {
			return false
		}
		defer img.File.Close()
		return img.Type == image.SIF
	}

	sandboxMarker, err := os.Stat(filepath.Join(path, ".singularity.d"))
	return err == nil && sandboxMarker.IsDir()
}

// FindBaseImage searches the given list of paths for a base container image
// corresponding to hash.  Each path is checked in turn in this way:
//   - if the entry itself is a SIF file or a sandbox directory, it is used
//     directly, regardless of hash.
//   - otherwise, if the entry is a directory, it is searched for either a
//     file/directory directly named hash, or for a file/directory named
//     hash within a subdirectory named with the first two characters of
//     hash (a "fan-out" layout, useful for distributing a large number of
//     base images).  If the hash is prefixed with a type followed by a colon
//     (e.g. "sha256:"), it is tried first with the prefix and then without.
//     Within the directory it is tried both without and with the first two
//     characters.
//
// The first matching path found is returned. If no match is found in any of
// the paths, an error is returned.
func FindBaseImage(hash string, paths []string) (string, error) {
	for _, p := range paths {
		if p == "" {
			continue
		}

		if isImage(p) {
			return p, nil
		}

		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}

		if hash == "" {
			continue
		}

		// split off any prefix with a colon
		prefix, h, found := strings.Cut(hash, ":")
		if found {
			prefix = prefix + ":"
		} else {
			h = prefix
			prefix = ""
		}

		var candidate string
		if prefix != "" {
			candidate = filepath.Join(p, prefix+h)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		candidate = filepath.Join(p, h)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		if len(h) > 2 {
			if prefix != "" {
				candidate = filepath.Join(p, prefix+h[:2], h[2:])
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
				candidate = filepath.Join(p, prefix+h[:2], h)
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
			candidate = filepath.Join(p, h[:2], h[2:])
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			candidate = filepath.Join(p, h[:2], h)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("could not find base image for hash %q in basepath %v", hash, paths)
}

// ResolveRemoteBase combines a --basepath remote reference with the base
// image reference recorded in an overlay image's metadata.
// The basepath must be a transport alone ("oras://" or "docker://") or a
// transport followed by a parseable OCI registry reference.  To indicate that
// a registry should come from the overlay image's default, use a registry of
// "-.-".  To indicate that a repository should be likewise from the default,
// use a repository of "-".  To use the default digest, leave it off the
// reference.  So for example "oras://" is equivalent to "oras://-.-/-".
func ResolveRemoteBase(uri string, labels map[string]string) (string, error) {
	transport, ref, ok := strings.Cut(uri, "://")
	if !ok || (transport != "oras" && transport != "docker") {
		return "", fmt.Errorf("unsupported remote basepath %q", uri)
	}
	if labels[overlayBootstrapLabel] != transport {
		return "", fmt.Errorf("basepath transport %q does not match overlay bootstrap transport %q", transport, labels[overlayBootstrapLabel])
	}

	from := strings.TrimSpace(labels[overlayFromLabel])
	hash := strings.TrimSpace(labels[overlayHashLabel])
	if from == "" || hash == "" {
		return "", fmt.Errorf("overlay image metadata is missing its bootstrap source or base hash")
	}
	if _, fromRef, ok := strings.Cut(from, "://"); ok {
		from = fromRef
	}
	sourceRef, err := name.ParseReference(from)
	if err != nil {
		return "", fmt.Errorf("invalid overlay bootstrap source: %w", err)
	}
	defaultRef, err := name.ParseReference(sourceRef.Context().Name() + "@" + hash)
	if err != nil {
		return "", fmt.Errorf("invalid default base image reference: %w", err)
	}
	if ref == "" {
		return transport + "://" + defaultRef.String(), nil
	}
	baseRef, err := name.ParseReference(ref)
	if err != nil {
		return "", fmt.Errorf("invalid remote basepath reference: %w", err)
	}

	defaultRepo := defaultRef.Context()
	baseRepo := baseRef.Context()
	registry := defaultRepo.RegistryStr()
	baseRegistry := baseRepo.RegistryStr()
	if baseRegistry != "-.-" {
		registry = baseRegistry
	}

	repository := defaultRepo.RepositoryStr()
	baseRepository := baseRepo.RepositoryStr()
	if baseRepository != "-" && baseRepository != "library/-" {
		repository = baseRepository
	}

	identifier := defaultRef.Identifier()
	sep := "@" // the defaultRef is always a digest
	baseIdentifier := baseRef.Identifier()
	if baseIdentifier != "-" && (baseIdentifier != "latest" || strings.HasSuffix(from, ":latest")) {
		identifier = baseIdentifier
		switch baseRef.(type) {
		case name.Tag:
			sep = ":"
		}
	}

	reference := registry + "/" + repository + sep + identifier
	return transport + "://" + reference, nil
}
