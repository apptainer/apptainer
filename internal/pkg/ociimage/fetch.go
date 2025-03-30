// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociimage

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/cache"
	progressClient "github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/ccoveille/go-safecast"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// cachedImage will ensure that the provided v1.Image is present in the Apptainer
// OCI cache layout dir, and return a new v1.Image pointing to the cached copy.
func cachedImage(ctx context.Context, imgCache *cache.Handle, srcImg v1.Image) (v1.Image, error) {
	if imgCache == nil || imgCache.IsDisabled() {
		return nil, fmt.Errorf("undefined image cache")
	}

	digest, err := srcImg.Digest()
	if err != nil {
		return nil, err
	}

	layoutDir, err := imgCache.GetOciCacheDir(cache.OciBlobCacheType)
	if err != nil {
		return nil, err
	}

	cachedRef := layoutDir + "@" + digest.String()
	sylog.Debugf("Caching image to %s", cachedRef)
	if err := OCISourceSink.WriteImage(srcImg, layoutDir, nil); err != nil {
		return nil, err
	}

	return OCISourceSink.Image(ctx, cachedRef, nil, nil)
}

// FetchToLayout will fetch the OCI image specified by imageRef to an OCI layout
// and return a v1.Image referencing it. If imgCache is non-nil, and enabled,
// the image will be fetched into Apptainer's cache - which is a multi-image
// OCI layout. If the cache is disabled, the image will be fetched into a
// subdirectory of the provided tmpDir. The caller is responsible for cleaning
// up tmpDir.
func FetchToLayout(ctx context.Context, tOpts *TransportOptions, imgCache *cache.Handle, imageURI, tmpDir string) (ggcrv1.Image, error) {
	// oci-archive - Perform a tar extraction first, and handle as an oci layout.
	if strings.HasPrefix(imageURI, "oci-archive:") {
		var tmpDir string
		tmpDir, err := os.MkdirTemp(tOpts.TmpDir, "temp-oci-")
		if err != nil {
			return nil, fmt.Errorf("could not create temporary oci directory: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// oci-archive:<path>[:tag]
		refParts := strings.SplitN(imageURI, ":", 3)
		sylog.Debugf("Extracting oci-archive %q to %q", refParts[1], tmpDir)
		err = extractArchive(refParts[1], tmpDir)
		if err != nil {
			return nil, fmt.Errorf("error extracting the OCI archive file: %v", err)
		}
		// We may or may not have had a ':tag' in the source to handle
		imageURI = "oci:" + tmpDir
		if len(refParts) == 3 {
			imageURI = imageURI + ":" + refParts[2]
		}
	}

	srcType, srcRef, err := URItoSourceSinkRef(imageURI)
	if err != nil {
		return nil, err
	}

	rt := progressClient.NewRoundTripper(ctx, nil)

	srcImg, err := srcType.Image(ctx, srcRef, tOpts, rt)
	if err != nil {
		rt.ProgressShutdown()
		return nil, err
	}

	if imgCache != nil && !imgCache.IsDisabled() {
		// Ensure the image is cached, and return reference to the cached image.
		cachedImg, err := cachedImage(ctx, imgCache, srcImg)
		if err != nil {
			rt.ProgressShutdown()
			return nil, err
		}
		rt.ProgressComplete()
		rt.ProgressWait()
		return cachedImg, nil
	}

	// No cache - write to layout directory provided
	tmpLayout, err := os.MkdirTemp(tmpDir, "layout-")
	if err != nil {
		return nil, err
	}
	sylog.Debugf("Copying %q to temporary layout at %q", srcRef, tmpLayout)
	if err = OCISourceSink.WriteImage(srcImg, tmpLayout, nil); err != nil {
		rt.ProgressShutdown()
		return nil, err
	}
	rt.ProgressComplete()
	rt.ProgressWait()

	return OCISourceSink.Image(ctx, tmpLayout, tOpts, nil)
}

// Perform a dumb tar(gz) extraction with no chown, id remapping etc.
// This is needed for non-root handling of `oci-archive` as the extraction
// by containers/archive is failing when uid/gid don't match local machine
// and we're not root
func extractArchive(src string, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	header, err := r.Peek(10) // read a few bytes without consuming
	if err != nil {
		return err
	}
	gzipped := strings.Contains(http.DetectContentType(header), "x-gzip")

	if gzipped {
		r, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer r.Close()
	}

	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// ZipSlip protection - don't escape from dst
		//#nosec G305
		target := filepath.Join(dst, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal extraction path", target)
		}

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			}
		// if it's a file create it
		case tar.TypeReg:
			tarMode, err := safecast.ToUint32(header.Mode)
			if err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(tarMode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			for {
				if _, err := io.CopyN(f, tr, 1024); err != nil {
					if err == io.EOF {
						break
					}
					return err
				}
			}
		}
	}
}
