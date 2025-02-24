// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package oci provides transparent caching of oci-like refs
package oci

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
)

// ImageReference wraps containers/image ImageReference type
type ImageReference struct {
	source types.ImageReference
	types.ImageReference
}

type GoArch struct {
	Arch string
	Var  string
}

var ArchMap = map[string]GoArch{
	"amd64": {
		Arch: "amd64",
		Var:  "",
	},
	"arm32v5": {
		Arch: "arm",
		Var:  "v5",
	},
	"arm32v6": {
		Arch: "arm",
		Var:  "v6",
	},
	"arm32v7": {
		Arch: "arm",
		Var:  "v7",
	},
	"arm64v8": {
		Arch: "arm64",
		Var:  "v8",
	},
	"386": {
		Arch: "386",
		Var:  "",
	},
	"ppc64le": {
		Arch: "ppc64le",
		Var:  "",
	},
	"s390x": {
		Arch: "s390x",
		Var:  "",
	},
	"riscv64": {
		Arch: "riscv64",
		Var:  "",
	},
}

// ConvertReference converts a source reference into a cache.ImageReference to cache its blobs
func ConvertReference(ctx context.Context, imgCache *cache.Handle, src types.ImageReference, topts *ociimage.TransportOptions) (types.ImageReference, error) {
	if imgCache == nil {
		return nil, fmt.Errorf("undefined image cache")
	}

	if topts == nil {
		// nolint:staticcheck
		topts = ociimage.TransportOptionsFromSystemContext(nil)
	}

	// Our cache dir is an OCI directory. We are using this as a 'blob pool'
	// storing all incoming containers under unique tags, which are a hash of
	// their source URI.
	cacheTag, err := getRefDigest(ctx, src, topts)
	if err != nil {
		return nil, err
	}

	cacheDir, err := imgCache.GetOciCacheDir(cache.OciBlobCacheType)
	if err != nil {
		return nil, err
	}
	c, err := layout.ParseReference(cacheDir + ":" + cacheTag)
	if err != nil {
		return nil, err
	}

	return &ImageReference{
		source:         src,
		ImageReference: c,
	}, nil
}

// NewImageSource wraps the cache's oci-layout ref to first download the real source image to the cache
func (t *ImageReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	return t.newImageSource(ctx, sys, sylog.Writer())
}

func (t *ImageReference) newImageSource(ctx context.Context, sys *types.SystemContext, w io.Writer) (types.ImageSource, error) {
	policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}

	// First we are fetching into the cache
	_, err = copy.Image(ctx, policyCtx, t.ImageReference, t.source, &copy.Options{
		ReportWriter:     w,
		SourceCtx:        sys,
		RemoveSignatures: true,
	})
	if err != nil {
		return nil, err
	}
	return t.ImageReference.NewImageSource(ctx, sys)
}

// ParseImageName parses a uri (e.g. docker://ubuntu) into it's transport:reference
// combination and then returns the proper reference
func ParseImageName(ctx context.Context, imgCache *cache.Handle, uri string, topts *ociimage.TransportOptions) (types.ImageReference, error) {
	ref, _, err := parseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("unable to parse image name %v: %v", uri, err)
	}

	return ConvertReference(ctx, imgCache, ref, topts)
}

func parseURI(uri string) (types.ImageReference, *GoArch, error) {
	sylog.Debugf("Parsing %s into reference", uri)

	arch := getArchFromURI(uri)

	split := strings.SplitN(uri, ":", 2)
	if len(split) != 2 {
		return nil, arch, fmt.Errorf("%s not in transport:reference pair", uri)
	}

	transport := transports.Get(split[0])
	if transport == nil {
		return nil, arch, fmt.Errorf("%s not a registered transport", split[0])
	}

	imgRef, err := transport.ParseReference(split[1])
	return imgRef, arch, err
}

// ImageDigest obtains the digest of a uri's manifest
func ImageDigest(ctx context.Context, uri string, topts *ociimage.TransportOptions) (digest string, err error) {
	ref, arch, err := parseURI(uri)
	if err != nil {
		return "", fmt.Errorf("unable to parse image name %v: %v", uri, err)
	}

	if arch != nil && arch.Arch != topts.Platform.Architecture {
		sylog.Warningf("The `--arch` value: %s is not equal to the arch info extracted from uri: %s, will ignore the `--arch` value", topts.Platform.Architecture, arch)
		topts.Platform.Architecture = arch.Arch
		topts.Platform.Variant = arch.Var
	}
	return getRefDigest(ctx, ref, topts)
}

// getRefDigest obtains the manifest digest for a ref.
func getRefDigest(ctx context.Context, ref types.ImageReference, topts *ociimage.TransportOptions) (digest string, err error) {
	// Handle docker references specially, using a HEAD request to ensure we don't hit API limits
	if ref.Transport().Name() == "docker" {
		digest, err := getDockerRefDigest(ctx, ref, topts)
		if err == nil {
			sylog.Debugf("GetManifest digest for %s is %s", transports.ImageName(ref), digest)
			return digest, err
		}
		// Need to have a fallback path, as the Docker-Content-Digest header is
		// not required in oci-distribution-spec.
		sylog.Debugf("Falling back to GetManifest digest: %s", err)
	}

	// Otherwise get the manifest and calculate sha256 over it
	// nolint:staticcheck
	source, err := ref.NewImageSource(ctx, ociimage.SystemContextFromTransportOptions(topts))
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = fmt.Errorf("%w (src: %v)", err, closeErr)
		}
	}()

	man, _, err := source.GetManifest(ctx, nil)
	if err != nil {
		return "", err
	}

	digest = fmt.Sprintf("%x", sha256.Sum256(man))
	digest = fmt.Sprintf("%x", sha256.Sum256([]byte(digest+topts.Platform.Architecture+topts.Platform.Variant)))
	sylog.Debugf("GetManifest digest for %s is %s", transports.ImageName(ref), digest)
	return digest, nil
}

// getDockerRefDigest obtains the manifest digest for a docker ref.
func getDockerRefDigest(ctx context.Context, ref types.ImageReference, topts *ociimage.TransportOptions) (digest string, err error) {
	// nolint:staticcheck
	d, err := docker.GetDigest(ctx, ociimage.SystemContextFromTransportOptions(topts), ref)
	if err != nil {
		return "", err
	}
	digest = d.Encoded()
	sylog.Debugf("docker.GetDigest source image digest for %s is %s", transports.ImageName(ref), digest)
	digest = fmt.Sprintf("%x", sha256.Sum256([]byte(digest+topts.Platform.Architecture+topts.Platform.Variant)))
	sylog.Debugf("docker.GetDigest digest for %s is %s", transports.ImageName(ref), digest)
	return digest, nil
}

func getArchFromURI(uri string) (arch *GoArch) {
	arch = nil
	split := strings.SplitN(uri, ":", 2)
	if len(split) != 2 {
		return
	}
	archURI := ""
	uriTmp := strings.TrimPrefix(split[1], "//")
	uriComponents := strings.Split(uriTmp, "/")
	// handle this type: docker://amd64/alpine
	if len(uriComponents) > 0 {
		archURI = uriComponents[0]
	}

	// handle this type: docker://docker.io/amd64/alpine
	if strings.IndexByte(archURI, '.') != -1 && len(uriComponents) > 1 {
		archURI = uriComponents[1]
	}

	val, ok := ArchMap[archURI]
	if ok {
		arch = &val
	}

	return
}

// Convert CLI options GOARCH and arch variant to recognized docker arch
func ConvertArch(arch, archVariant string) (string, error) {
	supportedArchs := []string{"arm", "arm64", "amd64", "386", "ppc64le", "s390x", "riscv64"}
	switch arch {
	case "arm64":
		if archVariant == "" {
			return "arm64v8", nil
		}
		tmpArch := ""
		if strings.HasPrefix(archVariant, "v") {
			tmpArch = fmt.Sprintf("%s%s", arch, archVariant)
		} else {
			tmpArch = fmt.Sprintf("%sv%s", arch, archVariant)
		}
		// verification
		if _, ok := ArchMap[tmpArch]; !ok {
			return "", fmt.Errorf("arch: %s is not valid, supported archs are: %v, supported variants are [8], please remove --arch-variant option", tmpArch, supportedArchs)
		}
		return tmpArch, nil
	case "arm":
		if archVariant == "" {
			armVal, ok := os.LookupEnv("GOARM")
			if !ok {
				return "", fmt.Errorf("arch: %s needs variant specification, supported variants are [5, 6, 7], please set --arch-variant option", arch)
			}
			archVariant = armVal
		}
		tmpArch := ""
		if strings.HasPrefix(archVariant, "v") {
			tmpArch = fmt.Sprintf("arm32%s", archVariant)
		} else {
			tmpArch = fmt.Sprintf("arm32v%s", archVariant)
		}
		// verification
		if _, ok := ArchMap[tmpArch]; !ok {
			return "", fmt.Errorf("arch: %s is not valid, supported archs are: %v, supported variants are [5, 6, 7]", tmpArch, supportedArchs)
		}
		return tmpArch, nil
	default:
		if _, ok := ArchMap[arch]; !ok {
			return "", fmt.Errorf("arch: %s is not valid, supported archs are: %v", arch, supportedArchs)
		}

		return arch, nil
	}
}
