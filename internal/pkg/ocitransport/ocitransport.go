// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ocitransport

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/apptainer/apptainer/pkg/util/slice"
	"github.com/containers/image/v5/docker"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	dockerdaemon "github.com/containers/image/v5/docker/daemon"
	ociarchive "github.com/containers/image/v5/oci/archive"
	ocilayout "github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var ociTransports = []string{"docker", "docker-archive", "docker-daemon", "oci", "oci-archive"}

// SupportedTransport returns whether or not the transport given is supported. To fit within a switch/case
// statement, this function will return transport if it is supported
func SupportedTransport(transport string) string {
	if slice.ContainsString(ociTransports, transport) {
		return transport
	}
	return ""
}

// TransportOptions provides authentication, platform etc. configuration for
// interactions with image transports.
type TransportOptions struct {
	// AuthConfig provides optional credentials to be used when interacting with
	// an image transport.
	AuthConfig *authn.AuthConfig
	// AuthFilePath provides an optional path to a file containing credentials
	// to be used when interacting with an image transport.
	AuthFilePath string
	// Insecure should be set to true in order to interact with a registry via
	// http, or without TLS certificate verification.
	Insecure bool
	// DockerDaemonHost provides the URI to use when interacting with a Docker
	// daemon.
	DockerDaemonHost string
	// Platform specifies the OS / Architecture / Variant that the pulled images
	// should satisfy.
	Platform v1.Platform
	// UserAgent will be set on HTTP(S) request made by transports.
	UserAgent string
	// TmpDir is a location in which a transport can create temporary files.
	TmpDir string
}

// SystemContext returns a containers/image/v5 types.SystemContext struct for
// compatibility with operations that still use containers/image.
//
// Deprecated: for containers/image compatibility only. To be removed in the future
func (t *TransportOptions) SystemContext() types.SystemContext {
	sc := types.SystemContext{
		AuthFilePath:            t.AuthFilePath,
		BigFilesTemporaryDir:    t.TmpDir,
		DockerRegistryUserAgent: t.UserAgent,
		OSChoice:                t.Platform.OS,
		ArchitectureChoice:      t.Platform.Architecture,
		VariantChoice:           t.Platform.Variant,
		DockerDaemonHost:        t.DockerDaemonHost,
	}

	if t.AuthConfig != nil {
		sc.DockerAuthConfig = &types.DockerAuthConfig{
			Username:      t.AuthConfig.Username,
			Password:      t.AuthConfig.Password,
			IdentityToken: t.AuthConfig.IdentityToken,
		}
	}

	if t.Insecure {
		sc.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
		sc.DockerDaemonInsecureSkipTLSVerify = true
		sc.OCIInsecureSkipTLSVerify = true
	}

	if sc.OSChoice == "" || sc.ArchitectureChoice == "" {
		// set default architecture and variant
		defaultSys := defaultSysCtx()
		if sc.OSChoice == "" {
			sc.OSChoice = defaultSys.OSChoice
		}
		if sc.ArchitectureChoice == "" {
			sc.ArchitectureChoice = defaultSys.ArchitectureChoice
			sc.VariantChoice = defaultSys.VariantChoice
		}
	}

	return sc
}

// TransportOptionsFromSystemContext returns a TransportOptions struct
// initialized from a containers/image SystemContext.
//
// Deprecated: for containers/image compatibility only. To be removed in future
func TransportOptionsFromSystemContext(sc *types.SystemContext) *TransportOptions {
	if sc == nil {
		sc = defaultSysCtx()
	}

	if sc.OSChoice == "" || sc.ArchitectureChoice == "" {
		// set default architecture and variant
		defaultSys := defaultSysCtx()
		if sc.OSChoice == "" {
			sc.OSChoice = defaultSys.OSChoice
		}
		if sc.ArchitectureChoice == "" {
			sc.ArchitectureChoice = defaultSys.ArchitectureChoice
			sc.VariantChoice = defaultSys.VariantChoice
		}
	}

	tOpts := TransportOptions{
		AuthFilePath: sc.AuthFilePath,
		TmpDir:       sc.BigFilesTemporaryDir,
		UserAgent:    sc.DockerRegistryUserAgent,
		Platform: v1.Platform{
			OS:           sc.OSChoice,
			Architecture: sc.ArchitectureChoice,
			Variant:      sc.VariantChoice,
		},
		Insecure: sc.DockerInsecureSkipTLSVerify == types.OptionalBoolTrue || sc.DockerDaemonInsecureSkipTLSVerify || sc.OCIInsecureSkipTLSVerify,
	}

	if sc.DockerAuthConfig != nil {
		tOpts.AuthConfig = &authn.AuthConfig{
			Username:      sc.DockerAuthConfig.Username,
			Password:      sc.DockerAuthConfig.Password,
			IdentityToken: sc.DockerAuthConfig.IdentityToken,
		}
	}

	return &tOpts
}

// SystemContextFromTransportOptions returns a containers/image SystemContext
// initialized from a TransportOptions struct. If the TrasnportOptions is nil,
// then nil is returned.
//
// Deprecated: for containers/image compatibility only. To be removed in future
func SystemContextFromTransportOptions(tOpts *TransportOptions) *types.SystemContext {
	if tOpts == nil {
		return nil
	}
	sc := tOpts.SystemContext()
	return &sc
}

// defaultPolicy is Apptainer's default containers/image OCI signature verification policy - accept anything.
func DefaultPolicy() (*signature.PolicyContext, error) {
	policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	return signature.NewPolicyContext(policy)
}

// parseImageRef parses a uri-like OCI image reference into a containers/image types.ImageReference.
func ParseImageRef(imageRef string) (types.ImageReference, error) {
	parts := strings.SplitN(imageRef, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("could not parse image ref: %s", imageRef)
	}

	var srcRef types.ImageReference
	var err error

	switch parts[0] {
	case "docker":
		srcRef, err = docker.ParseReference(parts[1])
	case "docker-archive":
		srcRef, err = dockerarchive.ParseReference(parts[1])
	case "docker-daemon":
		srcRef, err = dockerdaemon.ParseReference(parts[1])
	case "oci":
		srcRef, err = ocilayout.ParseReference(parts[1])
	case "oci-archive":
		srcRef, err = ociarchive.ParseReference(parts[1])
	default:
		return nil, fmt.Errorf("cannot create an OCI container from %s source", parts[0])
	}
	if err != nil {
		return nil, fmt.Errorf("invalid image source: %v", err)
	}

	return srcRef, nil
}

func defaultSysCtx() *types.SystemContext {
	sysCtx := &types.SystemContext{
		OSChoice: "linux",
	}
	switch runtime.GOARCH {
	case "arm64":
		sysCtx.ArchitectureChoice = runtime.GOARCH
		sysCtx.VariantChoice = "v8"
	case "arm":
		if variance, ok := os.LookupEnv("GOARM"); ok {
			sysCtx.ArchitectureChoice = runtime.GOARCH
			sysCtx.VariantChoice = "v" + variance
		} else {
			// by default, we are using arm32v7
			sysCtx.ArchitectureChoice = runtime.GOARCH
			sysCtx.VariantChoice = "v7"
		}
	default:
	}
	return sysCtx
}
