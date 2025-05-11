// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ipfs

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/client"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

// Timeout for an image pull in seconds - could be a large download...
const pullTimeout = 1800

// IsIpfsPullRef returns true if the provided string is a valid url
// reference for a pull operation.
func IsIpfsPullRef(netRef string) bool {
	match, _ := regexp.MatchString("^ipfs://b[a-z2-7]+", netRef)
	return match
}

// ipfsGateway returns the HTTP gateway for IPFS,
// e.g. "http://127.0.0.1:8080" for a local gateway
func ipfsGateway() (string, error) {
	gateway := os.Getenv("IPFS_GATEWAY")
	if gateway == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		file := filepath.Join(home, ".ipfs", "gateway")
		data, err := os.ReadFile(file)
		if err != nil {
			data, err = os.ReadFile("/etc/ipfs/gateway")
			if err != nil {
				return "", fmt.Errorf("no IPFS gateway found")
			}
		}
		gateway = string(data)
	}
	return gateway, nil
}

// DownloadImage will retrieve an image from an ipfs URI,
// saving it into the specified file
func DownloadImage(ctx context.Context, filePath string, ipfsURL string, outCid *string) error {
	if !IsIpfsPullRef(ipfsURL) {
		return fmt.Errorf("not a valid url reference: %s", ipfsURL)
	}
	if filePath == "" {
		refParts := strings.Split(ipfsURL, "/")
		filePath = refParts[len(refParts)-1]
		sylog.Infof("Download filename not provided. Downloading to: %s\n", filePath)
	}

	gateway, err := ipfsGateway()
	if err != nil {
		return err
	}
	addr := strings.Replace(ipfsURL, "ipfs://", "", 1)
	url := gateway + "/ipfs/" + addr
	sylog.Debugf("Pulling from URL: %s\n", url)

	httpClient := &http.Client{
		Timeout: pullTimeout * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", useragent.Value())

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("the requested image was not found")
	}

	if res.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(res.Body)
		s := buf.String()
		return fmt.Errorf("Download did not succeed: %d %s\n\t",
			res.StatusCode, s)
	}

	if etag := res.Header.Get("ETag"); etag != "" {
		sylog.Debugf("ETag: %s", etag)
		if outCid != nil {
			*outCid = strings.Replace(etag, "\"", "", -1)
		}
	}

	sylog.Debugf("OK response received, beginning body download\n")

	// Perms are 777 *prior* to umask
	out, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o777)
	if err != nil {
		return err
	}
	defer out.Close()

	pb := client.ProgressBarCallback(ctx)

	err = pb(res.ContentLength, res.Body, out)
	if err != nil {
		// Delete incomplete image file in the event of failure
		// we get here e.g. if the context is canceled by Ctrl-C
		res.Body.Close()
		out.Close()
		sylog.Infof("Cleaning up incomplete download: %s", filePath)
		if err := os.Remove(filePath); err != nil {
			sylog.Errorf("Error while removing incomplete download: %v", err)
		}
		return err
	}

	sylog.Debugf("Download complete\n")

	return nil
}

// decodeCID will decode the sha256 data of a CIDv1, while verifying the syntax of the multiformat string.
func decodeCID(cid string) ([]byte, error) {
	if cid[0] != 'b' { // multibase: base32
		return nil, fmt.Errorf("unknown encoding for cid: %c", cid[0])
	}
	str := strings.ToUpper(cid[1:])
	data, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(str)
	if err != nil {
		return nil, err
	}
	if len(data) != 4+256/8 {
		return nil, fmt.Errorf("wrong length for data: %d", len(data))
	}
	if data[0] != 1 { // cid: v1
		return nil, fmt.Errorf("unknown version for cid: %d", data[0])
	}
	if data[1] != 0x70 { // multicodec: dag-pb
		return nil, fmt.Errorf("unknown codec for cid: 0x%x", data[1])
	}
	if data[2] != 0x12 { // multihash code: sha2
		return nil, fmt.Errorf("unknown code for hash: 0x%x", data[2])
	}
	if data[3] != 32 { // multihash length: 256 bits
		return nil, fmt.Errorf("unknown length for hash: %d", data[3])
	}
	return data[4:], nil
}

// pull will pull a http(s) image into the cache if directTo="", or a specific file if directTo is set.
func pull(ctx context.Context, imgCache *cache.Handle, directTo, pullFrom string) (imagePath string, err error) {
	// We will cache using the sha256 from the URL, assuming that it is
	// a multibase base32 string containing a multihash sha2-256 string
	// of a multicodec merkle tree dag in protobuf format (i.e. a CIDv1)
	// If it is a directory, assume it only contains a single SIF file.
	// (that is: use the CID of the directory as the hash for the file,
	// this means that *all* files in it will use the same cache entry)
	cid := strings.Replace(pullFrom, "ipfs://", "", 1)
	cid = strings.Split(cid, "/")[0]
	cid = strings.Split(cid, "?")[0]

	sha, err := decodeCID(cid)
	if err != nil {
		return "", err
	}
	hash := hex.EncodeToString(sha)
	sylog.Debugf("Image hash for cache is: %s", hash)

	if directTo != "" {
		sylog.Infof("Downloading network image")
		if err := DownloadImage(ctx, directTo, pullFrom, nil); err != nil {
			return "", fmt.Errorf("unable to Download Image: %v", err)
		}
		imagePath = directTo

	} else {
		cacheEntry, err := imgCache.GetEntry(cache.IpfsCacheType, hash)
		if err != nil {
			return "", fmt.Errorf("unable to check if %v exists in cache: %v", hash, err)
		}
		defer cacheEntry.CleanTmp()

		if !cacheEntry.Exists {
			var filecid string

			sylog.Infof("Downloading network image")
			err := DownloadImage(ctx, cacheEntry.TmpPath, pullFrom, &filecid)
			if err != nil {
				sylog.Fatalf("%v\n", err)
			}

			err = cacheEntry.Finalize()
			if err != nil {
				return "", err
			}

			// if this cid is for the directory, then link it to the file cid
			if cid != filecid && filecid != "" {
				sha, err := decodeCID(filecid)
				if err != nil {
					return "", err
				}
				hash := hex.EncodeToString(sha)

				fileCacheEntry, err := imgCache.GetEntry(cache.IpfsCacheType, hash)
				if err != nil {
					return "", fmt.Errorf("unable to check if %v exists in cache: %v", hash, err)
				}
				defer fileCacheEntry.CleanTmp()
				err = os.Rename(cacheEntry.Path, fileCacheEntry.Path)
				if err != nil {
					return "", err
				}
				err = os.Symlink(hash, cacheEntry.Path)
				if err != nil {
					return "", err
				}
			}

		} else {
			sylog.Verbosef("Using image from cache")
		}

		imagePath = cacheEntry.Path
	}

	return imagePath, nil
}

// Pull will pull a http(s) image to the cache or direct to a temporary file if cache is disabled
func Pull(ctx context.Context, imgCache *cache.Handle, pullFrom string, tmpDir string) (imagePath string, err error) {
	directTo := ""

	if imgCache.IsDisabled() {
		file, err := os.CreateTemp(tmpDir, "sbuild-tmp-cache-")
		if err != nil {
			return "", fmt.Errorf("unable to create tmp file: %v", err)
		}
		directTo = file.Name()
		sylog.Infof("Downloading library image to tmp cache: %s", directTo)
	}

	return pull(ctx, imgCache, directTo, pullFrom)
}

// PullToFile will pull an http(s) image to the specified location, through the cache, or directly if cache is disabled
func PullToFile(ctx context.Context, imgCache *cache.Handle, pullTo, pullFrom string, sandbox bool) (imagePath string, err error) {
	directTo := ""
	if imgCache.IsDisabled() {
		directTo = pullTo
		sylog.Debugf("Cache disabled, pulling directly to: %s", directTo)
	}

	src, err := pull(ctx, imgCache, directTo, pullFrom)
	if err != nil {
		return "", fmt.Errorf("error fetching image to cache: %v", err)
	}

	if directTo == "" && !sandbox {
		// mode is before umask if pullTo doesn't exist
		err = fs.CopyFileAtomic(src, pullTo, 0o777)
		if err != nil {
			return "", fmt.Errorf("error copying image out of cache: %v", err)
		}
	}

	if sandbox {
		if err := client.ConvertSifToSandbox(directTo, src, pullTo); err != nil {
			return "", err
		}
	}

	return pullTo, nil
}
