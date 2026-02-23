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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/inspect"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/term"
)

// DownloadImage downloads an image specified by an oci reference to a file using the included credentials
func DownloadImage(ctx context.Context, path, ref, arch string, ociAuth *authn.AuthConfig, noHTTPS bool, reqAuthFile string) error {
	rt := client.NewRoundTripper(ctx, nil)
	im, err := remoteImage(ctx, ref, arch, ociAuth, noHTTPS, rt, reqAuthFile)
	if err != nil {
		rt.ProgressShutdown()
		return err
	}

	// Check manifest to ensure we have an image as single layer
	//
	// We *don't* check the image config mediaType as prior versions of
	// Apptainer have not been consistent in setting this, and really all we
	// care about is that we are pulling a single SIF or SquashFS file.
	//
	manifest, err := im.Manifest()
	if err != nil {
		rt.ProgressShutdown()
		return err
	}
	if len(manifest.Layers) != 1 {
		return fmt.Errorf("ORAS image should have a single layer, found %d", len(manifest.Layers))
	}
	layer := manifest.Layers[0]
	if layer.MediaType != SifLayerMediaTypeV1 &&
		layer.MediaType != SifLayerMediaTypeProto &&
		layer.MediaType != GenericBinaryMediaType {
		rt.ProgressShutdown()
		return fmt.Errorf("invalid layer mediatype: %s", layer.MediaType)
	}

	// Retrieve image to a temporary OCI layout
	tmpDirFlag := ""
	if v := ctx.Value(TmpDirKey); v != nil {
		if s, ok := v.(string); ok {
			tmpDirFlag = s
		}
	}
	// if tmpDirFlag is still "", os.MkdirTemp will use the system default temp dir
	tmpDir, err := os.MkdirTemp(tmpDirFlag, "oras-tmp-")
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

	// Copy image blob out from layout to final location
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

	// Ensure that we have downloaded an image (SIF or SquashFS)
	if err := ensureImage(path); err != nil {
		// remove whatever we downloaded if it is not an image
		os.RemoveAll(path)
		return err
	}
	return nil
}

// UploadImage uploads the image specified by path and pushes it to the provided oci reference,
// it will use credentials if supplied
func UploadImage(ctx context.Context, path, ref, arch string, sif bool, ociAuth *authn.AuthConfig, noHTTPS bool, reqAuthFile string) error {
	// ensure that are uploading an image (SIF or SquashFS)
	if err := ensureImage(path); err != nil {
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

	cmt := types.MediaType(UnknownConfigMediaTypeV1)
	lmt := types.MediaType(GenericBinaryMediaType)
	if sif {
		cmt = SifConfigMediaTypeV1
		lmt = SifLayerMediaTypeV1
	}

	im, err := NewImageFromSIF(path, cmt, lmt)
	if err != nil {
		return err
	}

	platform := v1.Platform{
		Architecture: arch,
		OS:           "linux",
	}
	remoteOpts := []remote.Option{
		ociauth.AuthOptn(ociAuth, reqAuthFile),
		remote.WithUserAgent(useragent.Value()),
		remote.WithContext(ctx),
		remote.WithPlatform(platform),
	}
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

// ensureImage checks for a SIF image or SquashFS at filepath and returns an error if it is not, or an error is encountered
func ensureImage(filepath string) error {
	img, err := image.Init(filepath, false)
	if err != nil {
		return fmt.Errorf("could not open image %s for verification: %s", filepath, err)
	}
	defer img.File.Close()

	if img.Type != image.SIF && img.Type != image.SQUASHFS {
		return fmt.Errorf("%q is not a SIF or SquashFS", filepath)
	}

	return nil
}

// ImageExtension returns the extension to use for the image at filepath and returns an error if it is not a known image type
func ImageExtension(filepath string) (string, error) {
	img, err := image.Init(filepath, false)
	if err != nil {
		sylog.Fatalf("could not open image %s: %s", filepath, err)
	}
	defer img.File.Close()

	switch img.Type {
	case image.SIF:
		return ".sif", nil
	case image.SQUASHFS:
		return ".squashfs", nil
	default:
		return "", fmt.Errorf("%q is not a SIF or SquashFS", filepath)
	}
}

// RefHash returns the digest of the image layer of the OCI manifest for supplied ref
func RefHash(ctx context.Context, ref, arch string, ociAuth *authn.AuthConfig, noHTTPS bool, reqAuthFile string) (v1.Hash, error) {
	im, err := remoteImage(ctx, ref, arch, ociAuth, noHTTPS, nil, reqAuthFile)
	if err != nil {
		return v1.Hash{}, err
	}

	// Check manifest to ensure we have an image as single layer
	manifest, err := im.Manifest()
	if err != nil {
		return v1.Hash{}, err
	}
	if len(manifest.Layers) != 1 {
		return v1.Hash{}, fmt.Errorf("ORAS image should have a single layer, found %d", len(manifest.Layers))
	}
	layer := manifest.Layers[0]
	if layer.MediaType != SifLayerMediaTypeV1 &&
		layer.MediaType != SifLayerMediaTypeProto &&
		layer.MediaType != GenericBinaryMediaType {
		return v1.Hash{}, fmt.Errorf("invalid layer mediatype: %s", layer.MediaType)
	}

	hash := layer.Digest
	return hash, nil
}

// ImageCreated returns the created for a file
func ImageCreated(filepath string) (time.Time, error) {
	img, err := image.Init(filepath, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not open image %s for verification: %s", filepath, err)
	}
	defer img.File.Close()

	switch img.Type {
	case image.SIF:
		return imageCreatedSIF(filepath)
	case image.SQUASHFS:
		return imageCreatedSquashfs(filepath)
	default:
		return time.Time{}, fmt.Errorf("%q is not a SIF or SquashFS", filepath)
	}
}

func imageCreatedSIF(filePath string) (time.Time, error) {
	f, err := sif.LoadContainerFromPath(filePath, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return time.Time{}, err
	}
	defer f.UnloadContainer()

	d, err := f.GetDescriptors(sif.WithDataType(sif.DataGenericJSON))
	if err != nil {
		return time.Time{}, err
	}

	created := time.Now()
	for _, desc := range d {
		if desc.Name() != image.SIFDescInspectMetadataJSON {
			continue
		}

		metadata := new(inspect.Metadata)
		if err := json.NewDecoder(desc.GetReader()).Decode(metadata); err != nil {
			return time.Time{}, err
		}

		buildDate := metadata.Attributes.Labels["org.label-schema.build-date"]
		t, err := parseBuildDate(buildDate)
		if err != nil {
			return time.Time{}, err
		}
		created = t
	}
	return created, nil
}

func parseBuildDate(date string) (time.Time, error) {
	// time.Parse uses underscores with a special meaning, so replace with space
	buildTime := strings.ReplaceAll(date, "_", " ")
	buildFormat := "Monday 2 January 2006 15:04:05 MST"
	t, err := time.Parse(buildFormat, buildTime)
	if err != nil {
		return time.Time{}, err
	}
	// Try to convert ambiguous US timezone strings, otherwise parsing as UTC(!)
	tz, err := time.LoadLocation(usTimezone(t.Zone()))
	if err == nil {
		t, err = time.ParseInLocation(buildFormat, buildTime, tz)
		if err != nil {
			return time.Time{}, err
		}
	}
	return t, nil
}

func usTimezone(name string, offset int) string {
	if offset == 0 {
		switch name {
		case "CDT", "CST":
			return "US/Central"
		case "EDT", "EST":
			return "US/Eastern"
		case "MDT", "MST":
			return "US/Mountain"
		case "PDT", "PST":
			return "US/Pacific"
		}
	}
	return name
}

func imageCreatedSquashfs(filePath string) (time.Time, error) {
	unsquashfs, err := bin.FindBin("unsquashfs")
	if err != nil {
		return time.Time{}, err
	}

	out, err := exec.Command(unsquashfs, "-stat", filePath).Output()
	if err != nil {
		return time.Time{}, err
	}

	created := time.Now()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Creation or last append time ") {
			t, err := parseCreationTime(line)
			if err != nil {
				return time.Time{}, err
			}
			created = t
		}
	}
	return created, nil
}

func parseCreationTime(date string) (time.Time, error) {
	squashfsTime := strings.Replace(date, "Creation or last append time ", "", 1)
	squashfsFormat := "Mon Jan 2 15:04:05 2006"
	t, err := time.ParseInLocation(squashfsFormat, squashfsTime, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
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
func remoteImage(ctx context.Context, ref, arch string, ociAuth *authn.AuthConfig, noHTTPS bool, rt *client.RoundTripper, reqAuthFile string) (v1.Image, error) {
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
	platform := v1.Platform{
		Architecture: arch,
		OS:           "linux",
	}
	remoteOpts := []remote.Option{
		ociauth.AuthOptn(ociAuth, reqAuthFile),
		remote.WithContext(ctx),
		remote.WithPlatform(platform),
	}
	if rt != nil {
		remoteOpts = append(remoteOpts, remote.WithTransport(rt))
	}
	im, err := remote.Image(ir, remoteOpts...)
	if err != nil {
		return nil, err
	}
	return im, nil
}
