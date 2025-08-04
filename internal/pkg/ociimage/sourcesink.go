// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociimage

import (
	"context"
	"fmt"
	"strings"

	progressClient "github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type SourceSink int

const (
	UnknownSourceSink SourceSink = iota
	RegistrySourceSink
	OCISourceSink
	TarballSourceSink
	DaemonSourceSink
)

func getDockerImage(ctx context.Context, src string, tOpts *TransportOptions, rt *progressClient.RoundTripper) (v1.Image, error) {
	var nameOpts []name.Option
	if tOpts != nil && tOpts.Insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}

	srcRef, err := name.ParseReference(src, nameOpts...)
	if err != nil {
		return nil, err
	}

	// See if there's a mirror to use for this registry by applying
	// containers/image library functions.
	// This may one day be done automatically by go-containerregistry.
	// If that happens we can remove this code.
	// See https://github.com/apptainer/apptainer/issues/2919
	host := srcRef.Context().Registry.Name()
	var regctx types.SystemContext
	registry, _ := sysregistriesv2.FindRegistry(&regctx, host)
	if host == "index.docker.io" {
		// This is the default registry; if it failed to find a mirror,
		// instead try the equivalent shorter version that might be
		// defined with a mirror.
		host = "docker.io"
		if registry == nil {
			registry, _ = sysregistriesv2.FindRegistry(&regctx, host)
		}
	}
	if (registry != nil) && (len(registry.Mirrors) > 0) {
		mirror := registry.Mirrors[0].Location
		nameOpts = []name.Option{}
		if registry.Mirrors[0].Insecure {
			nameOpts = append(nameOpts, name.Insecure)
		}
		mirrorSrc := src
		// Normalize the src, for example by prefixing docker.io and
		// adding library/ for standard docker.io containers, because
		// mirrors expect this to already be done.
		normalizedRef, err := reference.ParseNormalizedNamed(mirrorSrc)
		if err != nil {
			sylog.Debugf("Normalizing %s failed, using as-is: %v", mirrorSrc, err)
		} else {
			mirrorSrc = normalizedRef.String()
		}
		// remove the first component if it was an explicit registry
		mirrorParts := strings.Split(mirrorSrc, "/")
		if (host != "docker.io") || strings.HasSuffix(mirrorParts[0], host) {
			// this should always happen unless normalizing
			// failed and the src is missing a registry name
			mirrorSrc = strings.Join(mirrorParts[1:], "/")
		}
		// then add the mirror in its place
		mirrorSrc = mirror + "/" + mirrorSrc
		sylog.Debugf("Using %s mirror in place of %s", mirrorSrc, src)
		mirrorRef, err := name.ParseReference(mirrorSrc, nameOpts...)
		if err != nil {
			sylog.Warningf("Error parsing registry mirror reference %s, skipping mirror: %v", mirrorSrc, err)
		} else {
			srcRef = mirrorRef
		}
	}

	pullOpts := []remote.Option{
		remote.WithContext(ctx),
	}

	if tOpts != nil {
		pullOpts = append(pullOpts,
			remote.WithPlatform(tOpts.Platform),
			ociauth.AuthOptn(tOpts.AuthConfig, tOpts.AuthFilePath))
	}

	if rt != nil {
		pullOpts = append(pullOpts, remote.WithTransport(rt))
	}

	return remote.Image(srcRef, pullOpts...)
}

// getOCIImage retrieves an image from a layout ref provided in <dir>[@digest] format.
// If no digest is provided, and there is only one image in the layout, it will be returned.
// A digest must be specified when retrieving an image from a layout containing multiple images.
func getOCIImage(src string, tOpts *TransportOptions) (v1.Image, error) {
	refParts := strings.SplitN(src, "@", 2)

	lp, err := layout.FromPath(refParts[0])
	if err != nil {
		return nil, err
	}

	ii, err := lp.ImageIndex()
	if err != nil {
		return nil, err
	}

	im, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}

	if len(im.Manifests) < 1 {
		return nil, fmt.Errorf("no images found in layout %s", src)
	}

	if len(refParts) < 2 && len(im.Manifests) != 1 {
		return nil, fmt.Errorf("must specify a digest - layout contains multiple images")
	}
	if len(refParts) == 1 {
		mf := im.Manifests[0]
		if mf.MediaType.IsIndex() {
			ii, err := ii.ImageIndex(mf.Digest)
			if err != nil {
				return nil, err
			}
			return getPlatformImage(ii, tOpts.Platform)
		}
		return lp.Image(mf.Digest)
	}

	for _, mf := range im.Manifests {
		sylog.Debugf("%v =? %v", mf.Digest.String(), refParts[1])
		if mf.Digest.String() == refParts[1] {
			return ii.Image(mf.Digest)
		}
	}

	return nil, fmt.Errorf("image %q not found in layout", src)
}

func getPlatformImage(ii v1.ImageIndex, platform v1.Platform) (v1.Image, error) {
	im, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}

	for _, mf := range im.Manifests {
		// skip attestation manifest
		if mf.Platform.OS == "unknown" && mf.Platform.Architecture == "unknown" {
			continue
		}
		sylog.Debugf("%v =? %v", mf.Digest.String(), mf.Platform.String())
		if mf.Platform.Satisfies(platform) {
			image, err := ii.Image(mf.Digest)
			if err != nil {
				return nil, err
			}
			// check that blob exists
			_, err = image.ConfigFile()
			if err != nil {
				return nil, fmt.Errorf("%s image not found in blobs", platform)
			}
			return image, nil
		}
	}

	return nil, fmt.Errorf("%s image not found in index", platform)
}

func getDaemonImage(ctx context.Context, src string, tOpts *TransportOptions) (v1.Image, error) {
	var nameOpts []name.Option
	if tOpts != nil && tOpts.Insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}

	srcRef, err := name.ParseReference(src, nameOpts...)
	if err != nil {
		return nil, err
	}

	dOpts := []daemon.Option{
		daemon.WithContext(ctx),
	}

	if tOpts != nil && tOpts.DockerDaemonHost != "" {
		dc, err := client.NewClientWithOpts(client.WithHost(tOpts.DockerDaemonHost))
		if err != nil {
			return nil, err
		}
		dOpts = append(dOpts, daemon.WithClient(dc))
	}

	return daemon.Image(srcRef, dOpts...)
}

func (ss SourceSink) Reference(s string, tOpts *TransportOptions) (name.Reference, bool) {
	switch ss {
	case RegistrySourceSink, DaemonSourceSink:
		var nameOpts []name.Option
		if tOpts != nil && tOpts.Insecure {
			nameOpts = append(nameOpts, name.Insecure)
		}
		srcRef, err := name.ParseReference(s, nameOpts...)
		if err != nil {
			return nil, false
		}
		return srcRef, true
	default:
		return nil, false
	}
}

func (ss SourceSink) Image(ctx context.Context, ref string, tOpts *TransportOptions, rt *progressClient.RoundTripper) (v1.Image, error) {
	switch ss {
	case RegistrySourceSink:
		return getDockerImage(ctx, ref, tOpts, rt)
	case TarballSourceSink:
		return tarball.ImageFromPath(ref, nil)
	case OCISourceSink:
		return getOCIImage(ref, tOpts)
	case DaemonSourceSink:
		return getDaemonImage(ctx, ref, tOpts)
	case UnknownSourceSink:
		return nil, errUnsupportedTransport
	default:
		return nil, errUnsupportedTransport
	}
}

func (ss SourceSink) WriteImage(img v1.Image, dstName string, tOpts *TransportOptions) error {
	switch ss {
	case OCISourceSink:
		lp, err := layout.FromPath(dstName)
		if err != nil {
			lp, err = layout.Write(dstName, empty.Index)
			if err != nil {
				return err
			}
		}
		return lp.AppendImage(img)

	case RegistrySourceSink:
		var nameOpts []name.Option
		if tOpts != nil && tOpts.Insecure {
			nameOpts = append(nameOpts, name.Insecure)
		}
		dstRef, err := name.ParseReference(dstName, nameOpts...)
		if err != nil {
			return err
		}
		remoteOpts := []remote.Option{}
		if tOpts != nil {
			remoteOpts = append(remoteOpts,
				remote.WithPlatform(tOpts.Platform),
				ociauth.AuthOptn(tOpts.AuthConfig, tOpts.AuthFilePath))
		}
		return remote.Write(dstRef, img, remoteOpts...)

	case TarballSourceSink:
		// Only supports writing a single image per tarball.
		dstRef := name.MustParseReference("image")
		return tarball.WriteToFile(dstName, dstRef, img)

	case UnknownSourceSink:
		return errUnsupportedTransport

	default:
		return errUnsupportedTransport
	}
}
