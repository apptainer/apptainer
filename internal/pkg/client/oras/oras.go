// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containers/image/v5/manifest"
	ocitypes "github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras_docker "oras.land/oras-go/pkg/auth/docker"
	"oras.land/oras-go/pkg/content"
	orasctx "oras.land/oras-go/pkg/context"
	"oras.land/oras-go/pkg/oras"
	orasAuth "oras.land/oras-go/pkg/registry/remote/auth"
)

const (
	// SifDefaultTag is the tag to use when a tag is not specified
	SifDefaultTag = "latest"

	// SifConfigMediaTypeV1 is the config descriptor mediaType
	// Since we only ever send a null config this should not have the
	// format extension appended:
	//   https://github.com/deislabs/oras/#pushing-artifacts-with-single-files
	//   If a null config is passed, the config extension must be removed.
	SifConfigMediaTypeV1 = "application/vnd.sylabs.sif.config.v1"

	// SifLayerMediaTypeV1 is the mediaType for the "layer" which contains the actual SIF file
	SifLayerMediaTypeV1 = "application/vnd.sylabs.sif.layer.v1.sif"

	// SifLayerMediaTypeProto is the mediaType from prototyping and Apptainer
	// <3.7 which unfortunately includes a typo and doesn't have a version suffix
	// See: https://github.com/apptainer/singularity/issues/4437
	SifLayerMediaTypeProto = "appliciation/vnd.sylabs.sif.layer.tar"
)

var sifLayerMediaTypes = []string{SifLayerMediaTypeV1, SifLayerMediaTypeProto}

type orasUploadTransport struct {
	rt    http.RoundTripper
	scope string
}

func (t *orasUploadTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	values := r.URL.Query()
	// service is a required parameter
	if values.Has("service") {
		// inspect scopes and merge them if possible
		if scopes, ok := values["scope"]; ok && len(scopes) > 1 {
			values["scope"] = orasAuth.CleanScopes(scopes)
			r.URL.RawQuery = values.Encode()
		}
	}
	return t.rt.RoundTrip(r)
}

func newOrasUploadTransport() http.RoundTripper {
	return &orasUploadTransport{
		rt: http.DefaultTransport,
	}
}

func getResolver(ctx context.Context, ociAuth *ocitypes.DockerAuthConfig, noHTTPS, push bool) (remotes.Resolver, error) {
	opts := docker.ResolverOptions{Credentials: genCredfn(ociAuth), PlainHTTP: noHTTPS}
	if ociAuth != nil && (ociAuth.Username != "" || ociAuth.Password != "") {
		return docker.NewResolver(opts), nil
	}

	cli, err := oras_docker.NewClient(syfs.DockerConf())
	if err != nil {
		sylog.Warningf("Couldn't load auth credential file: %s", err)
		return docker.NewResolver(opts), nil
	}

	httpClient := &http.Client{}

	// docker client doesn't merge scopes correctly and can set multiple scopes in url parameters when pushing image:
	// "scope=repository:my_namespace/alpine:pull&scope=repository:my_namespace:alpine:pull,push",
	// this could be merged to "scope=repository:my_namespace:alpine:pull,push".
	// Since there are authorization servers that might not support multiple scopes, a custom transport is injected
	// to merge duplicated scopes
	if push {
		httpClient.Transport = newOrasUploadTransport()
	}

	return cli.Resolver(ctx, httpClient, noHTTPS)
}

// DownloadImage downloads a SIF image specified by an oci reference to a file using the included credentials
func DownloadImage(ctx context.Context, imagePath, ref string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool) error {
	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")

	spec, err := reference.Parse(ref)
	if err != nil {
		return fmt.Errorf("unable to parse oci reference: %s", err)
	}

	// append default tag if no object exists
	if spec.Object == "" {
		spec.Object = SifDefaultTag
		sylog.Infof("No tag or digest found, using default: %s", SifDefaultTag)
	}

	resolver, err := getResolver(ctx, ociAuth, noHTTPS, false)
	if err != nil {
		return fmt.Errorf("while getting resolver: %s", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %s", err)
	}

	store := content.NewFile(wd)
	defer store.Close()

	store.AllowPathTraversalOnWrite = true
	// With image caching via download to tmpfile + rename we are now overwriting the temporary file that is created
	// so we have to allow an overwrite here.
	store.DisableOverwrite = false

	allowedMediaTypes := oras.WithAllowedMediaTypes(sifLayerMediaTypes)
	handlerFunc := func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		for _, mt := range sifLayerMediaTypes {
			if desc.MediaType == mt {
				// Ensure descriptor is of a single file
				// AnnotationUnpack indicates that the descriptor is of a directory
				if desc.Annotations[content.AnnotationUnpack] == "true" {
					return nil, fmt.Errorf("descriptor is of a bundled directory, not a SIF image")
				}
				nameOld, _ := content.ResolveName(desc)
				sylog.Debugf("Will pull oras image %s to %s", nameOld, imagePath)
				_ = store.MapPath(nameOld, imagePath)
			}
		}
		return nil, nil
	}
	pullHandler := oras.WithPullBaseHandler(images.HandlerFunc(handlerFunc))

	_, err = oras.Copy(orasctx.WithLoggerDiscarded(ctx), resolver, spec.String(), store, "", allowedMediaTypes, pullHandler)
	if err != nil {
		return fmt.Errorf("unable to pull from registry: %s", err)
	}

	// ensure that we have downloaded a SIF
	if err := ensureSIF(imagePath); err != nil {
		// remove whatever we downloaded if it is not a SIF
		os.RemoveAll(imagePath)
		return err
	}

	// ensure container is executable
	if err := os.Chmod(imagePath, 0o755); err != nil {
		return fmt.Errorf("unable to set image perms: %s", err)
	}

	return nil
}

// UploadImage uploads the image specified by path and pushes it to the provided oci reference,
// it will use credentials if supplied
func UploadImage(ctx context.Context, path, ref string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool) error {
	// ensure that are uploading a SIF
	if err := ensureSIF(path); err != nil {
		return err
	}

	ref = strings.TrimPrefix(ref, "oras://")
	ref = strings.TrimPrefix(ref, "//")

	spec, err := reference.Parse(ref)
	if err != nil {
		return fmt.Errorf("unable to parse oci reference: %w", err)
	}

	// Hostname() will panic if there is no '/' in the locator
	// explicitly check for this and fail in order to prevent panic
	// this case will only occur for incorrect uris
	if !strings.Contains(spec.Locator, "/") {
		return fmt.Errorf("not a valid oci object uri: %s", ref)
	}

	// append default tag if no object exists
	if spec.Object == "" {
		spec.Object = SifDefaultTag
		sylog.Infof("No tag or digest found, using default: %s", SifDefaultTag)
	}

	resolver, err := getResolver(ctx, ociAuth, noHTTPS, true)
	if err != nil {
		return fmt.Errorf("while getting resolver: %s", err)
	}

	store := content.NewFile("")
	defer store.Close()

	// Get the filename from path and use it as the name in the file store
	name := filepath.Base(path)

	desc, err := store.Add(name, SifLayerMediaTypeV1, path)
	if err != nil {
		return fmt.Errorf("unable to add SIF to store: %w", err)
	}

	manifest, manifestDesc, config, configDesc, err := content.GenerateManifestAndConfig(nil, nil, desc)
	if err != nil {
		return fmt.Errorf("unable to generate manifest and config: %w", err)
	}

	if err := store.Load(configDesc, config); err != nil {
		return fmt.Errorf("unable to load config: %w", err)
	}

	if err := store.StoreManifest("local", manifestDesc, manifest); err != nil {
		return fmt.Errorf("unable to store manifest: %w", err)
	}

	if _, err = oras.Copy(orasctx.WithLoggerDiscarded(ctx), store, "local", resolver, spec.String()); err != nil {
		return fmt.Errorf("unable to push: %w", err)
	}

	return nil
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

// ImageSHA returns the sha256 digest of the SIF layer of the OCI manifest
// oci spec dictates only sha256 and sha512 are supported at time creation for this function
// sha512 is currently optional for implementations, this function will return an error when
// encountering such digests.
// https://github.com/opencontainers/image-spec/blob/master/descriptor.md#registered-algorithms
func ImageSHA(ctx context.Context, uri string, ociAuth *ocitypes.DockerAuthConfig, noHTTPS bool) (string, error) {
	ref := strings.TrimPrefix(uri, "oras://")
	ref = strings.TrimPrefix(ref, "//")

	resolver, err := getResolver(ctx, ociAuth, noHTTPS, false)
	if err != nil {
		return "", fmt.Errorf("while getting resolver: %s", err)
	}

	_, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("while resolving reference: %v", err)
	}

	// ensure that we received an image manifest descriptor
	if desc.MediaType != ocispec.MediaTypeImageManifest {
		if desc.MediaType == manifest.DockerV2Schema2MediaType {
			return "", errors.New("please use docker:// instead of oras:// to pull the image")
		}
		return "", fmt.Errorf("could not get image manifest, received mediaType: %s", desc.MediaType)
	}

	fetcher, err := resolver.Fetcher(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("while creating fetcher for reference: %v", err)
	}

	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return "", fmt.Errorf("while fetching manifest: %v", err)
	}
	defer rc.Close()

	b, err := ioutil.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("while reading manifest: %v", err)
	}

	var man ocispec.Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		return "", fmt.Errorf("while unmarshalling manifest: %v", err)
	}

	// search image layers for sif image and return sha
	for _, l := range man.Layers {
		for _, t := range sifLayerMediaTypes {
			if l.MediaType == t {
				// only allow sha256 digests
				if l.Digest.Algorithm() != digest.SHA256 {
					return "", fmt.Errorf("SIF layer found with incorrect digest algorithm: %s", l.Digest.Algorithm())
				}
				return l.Digest.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no layer found corresponding to SIF image")
}

// ImageHash returns the appropriate hash for a provided image file
// e.g. sha256:<sha256>
func ImageHash(filePath string) (result string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	result, _, err = sha256sum(file)

	return result, err
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

func genCredfn(ociAuth *ocitypes.DockerAuthConfig) func(string) (string, string, error) {
	return func(_ string) (string, string, error) {
		if ociAuth != nil {
			return ociAuth.Username, ociAuth.Password, nil
		}

		return "", "", nil
	}
}
