// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociimage

import (
	"context"
	"errors"
	"fmt"

	"github.com/apptainer/apptainer/pkg/sylog"
	digest "github.com/opencontainers/go-digest"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/pkg/sysregistriesv2"
	"go.podman.io/image/v5/types"
)

// PullSources returns, in the order they should be tried, the locations to
// pull ref from according to registries.conf: the matching [[registry]]'s
// mirrors, then its (possibly prefix-rewritten) location. This is the same
// resolution containers/image applies in its own pull path, so apptainer
// contacts exactly the endpoints podman/skopeo would. If no [[registry]]
// matches, ref itself is the only source.
func PullSources(sys *types.SystemContext, ref reference.Named) ([]sysregistriesv2.PullSource, error) {
	registry, err := sysregistriesv2.FindRegistry(sys, ref.String())
	if err != nil {
		return nil, fmt.Errorf("loading registries configuration: %w", err)
	}
	if registry == nil {
		return []sysregistriesv2.PullSource{{Reference: ref}}, nil
	}
	if registry.Blocked {
		return nil, fmt.Errorf("registry %s is blocked in %s", registry.Prefix, sysregistriesv2.ConfigurationSourceDescription(sys))
	}
	return registry.PullSourcesFromReference(ref)
}

// DockerDigest is docker.GetDigest, honoring registries.conf mirrors and
// location rewrites via PullSources. docker.GetDigest on its own always
// contacts the registry named in ref.
func DockerDigest(ctx context.Context, sys *types.SystemContext, ref types.ImageReference) (digest.Digest, error) {
	named := ref.DockerReference()
	if named == nil {
		// nolint:staticcheck
		return docker.GetDigest(ctx, sys, ref)
	}
	sources, err := PullSources(sys, named)
	if err != nil {
		return "", err
	}
	errs := make([]error, 0, len(sources))
	for _, src := range sources {
		srcRef, err := docker.NewReference(src.Reference)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		srcSys := sys
		if src.Endpoint.Insecure {
			c := types.SystemContext{}
			if sys != nil {
				c = *sys
			}
			c.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
			srcSys = &c
		}
		// nolint:staticcheck
		d, err := docker.GetDigest(ctx, srcSys, srcRef)
		if err == nil {
			if src.Reference.String() != named.String() {
				sylog.Debugf("Digest for %s obtained from %s", named, src.Reference)
			}
			return d, nil
		}
		sylog.Debugf("Getting digest from %s failed: %v", src.Reference, err)
		errs = append(errs, fmt.Errorf("%s: %w", src.Reference, err))
	}
	return "", errors.Join(errs...)
}
