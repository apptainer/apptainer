// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	ocitypes "github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/term"
)

// DownloadImage downloads a SIF image specified by an oci reference to a file using the included credentials
func DownloadImage(ctx context.Context, path, ref string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool, reqAuthFile string) error {
	rt := client.NewRoundTripper(ctx, nil)
	im, err := remoteImage(ref, ociAuth, noHTTPS, rt, reqAuthFile)
	if err != nil {
		rt.ProgressShutdown()
		return err
	}

	// Check manifest to ensure we have a SIF as single layer
	//
	// We *don't* check the image config mediaType as prior versions of
	// Apptainer have not been consistent in setting this, and really all we
	// care about is that we are pulling a single SIF file.
	//
	manifest, err := im.Manifest()
	if err != nil {
		rt.ProgressShutdown()
		return err
	}
	if len(manifest.Layers) != 1 {
		return fmt.Errorf("ORAS SIF image should have a single layer, found %d", len(manifest.Layers))
	}
	layer := manifest.Layers[0]
	if layer.MediaType != SifLayerMediaTypeV1 &&
		layer.MediaType != SifLayerMediaTypeProto {
		rt.ProgressShutdown()
		return fmt.Errorf("invalid layer mediatype: %s", layer.MediaType)
	}

	// Retrieve image to a temporary OCI layout
	tmpDir, err := os.MkdirTemp("", "oras-tmp-")
	if err != nil {
		rt.ProgressShutdown()
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			sylog.Errorf("while removing %q: %v", tmpDir, err)
		}
	}()
	tmpLayout, err := layout.Write(tmpDir, empty.Index)
	if err != nil {
		rt.ProgressShutdown()
		return err
	}
	if err := tmpLayout.AppendImage(im); err != nil {
		rt.ProgressShutdown()
		return err
	}

	rt.ProgressComplete()
	rt.ProgressWait()

	// Copy SIF blob out from layout to final location
	blob, err := tmpLayout.Blob(layer.Digest)
	if err != nil {
		return err
	}
	defer blob.Close()
	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, blob)
	if err != nil {
		return err
	}

	// Ensure that we have downloaded a SIF
	if err := ensureSIF(path); err != nil {
		// remove whatever we downloaded if it is not a SIF
		os.RemoveAll(path)
		return err
	}
	return nil
}

// UploadImage uploads the image specified by path and pushes it to the provided oci reference,
// it will use credentials if supplied
func UploadImage(_ context.Context, path, ref string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool, reqAuthFile string) error {
	// ensure that are uploading a SIF
	if err := ensureSIF(path); err != nil {
		return err
	}

	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")

	// Get reference to image in the remote
	opts := []name.Option{name.WithDefaultTag(name.DefaultTag), name.WithDefaultRegistry(name.DefaultRegistry)}
	if noHTTPS {
		opts = append(opts, name.Insecure)
	}
	ir, err := name.ParseReference(ref, opts...)
	if err != nil {
		return err
	}

	im, err := NewImageFromSIF(path, SifLayerMediaTypeV1)
	if err != nil {
		return err
	}

	remoteOpts := []remote.Option{ociauth.AuthOptn(ociAuth, reqAuthFile), remote.WithUserAgent(useragent.Value())}
	if term.IsTerminal(2) {
		pb := &client.DownloadProgressBar{}
		progChan := make(chan v1.Update, 1)
		go func() {
			var total int64
			soFar := int64(0)
			for {
				// The following is concurrency-safe because this is the only
				// goroutine that's going to be reading progChan updates.
				update := <-progChan
				if update.Error != nil {
					pb.Abort(false)
					return
				}
				if update.Total != total {
					pb.Init(update.Total)
					total = update.Total
				}
				pb.IncrBy(int(update.Complete - soFar))
				soFar = update.Complete
				if soFar >= total {
					pb.Wait()
					return
				}
			}
		}()
		remoteOpts = append(remoteOpts, remote.WithProgress(progChan))
	}
	return remote.Write(ir, im, remoteOpts...)
}

// ensureSIF checks for a SIF image at filepath and returns an error if it is not, or an error is encountered
func ensureSIF(filepath string) error {
	img, err := image.Init(filepath, false)
	if err != nil {
		return fmt.Errorf("could not open image %s for verification: %s", filepath, err)
	}
	defer img.File.Close()

	if img.Type != image.SIF {
		return fmt.Errorf("%q is not a SIF", filepath)
	}

	return nil
}

// RefHash returns the digest of the SIF layer of the OCI manifest for supplied ref
func RefHash(_ context.Context, ref string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool, reqAuthFile string) (v1.Hash, error) {
	im, err := remoteImage(ref, ociAuth, noHTTPS, nil, reqAuthFile)
	if err != nil {
		return v1.Hash{}, err
	}

	// Check manifest to ensure we have a SIF as single layer
	manifest, err := im.Manifest()
	if err != nil {
		return v1.Hash{}, err
	}
	if len(manifest.Layers) != 1 {
		return v1.Hash{}, fmt.Errorf("ORAS SIF image should have a single layer, found %d", len(manifest.Layers))
	}
	layer := manifest.Layers[0]
	if layer.MediaType != SifLayerMediaTypeV1 &&
		layer.MediaType != SifLayerMediaTypeProto {
		return v1.Hash{}, fmt.Errorf("invalid layer mediatype: %s", layer.MediaType)
	}

	hash := layer.Digest
	return hash, nil
}

// ImageDigest returns the digest for a file
func ImageHash(filePath string) (v1.Hash, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return v1.Hash{}, err
	}
	defer file.Close()

	sha, _, err := sha256sum(file)
	if err != nil {
		return v1.Hash{}, err
	}

	hash, err := v1.NewHash(sha)
	if err != nil {
		return v1.Hash{}, err
	}

	return hash, nil
}

// sha256sum computes the sha256sum of the specified reader; caller is
// responsible for resetting file pointer. 'nBytes' indicates number of
// bytes read from reader
func sha256sum(r io.Reader) (result string, nBytes int64, err error) {
	hash := sha256.New()
	nBytes, err = io.Copy(hash, r)
	if err != nil {
		return "", 0, err
	}

	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nBytes, nil
}

// remoteImage returns a v1.Image for the provided remote ref.
func remoteImage(ref string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool, rt *client.RoundTripper, reqAuthFile string) (v1.Image, error) {
	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")

	// Get reference to image in the remote
	opts := []name.Option{name.WithDefaultTag(name.DefaultTag), name.WithDefaultRegistry(name.DefaultRegistry)}
	if noHTTPS {
		opts = append(opts, name.Insecure)
	}
	ir, err := name.ParseReference(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q: %w", ref, err)
	}
	remoteOpts := []remote.Option{ociauth.AuthOptn(ociAuth, reqAuthFile)}
	if rt != nil {
		remoteOpts = append(remoteOpts, remote.WithTransport(rt))
	}
	im, err := remote.Image(ir, remoteOpts...)
	if err != nil {
		return nil, err
	}
	return im, nil
}
