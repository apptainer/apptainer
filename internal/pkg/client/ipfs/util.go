// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ipfs

import (
	"encoding/base32"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

// decodeCID will decode the sha256 data of a CIDv1,
// while verifying the syntax of the multiformat string.
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
