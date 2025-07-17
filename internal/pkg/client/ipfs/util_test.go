// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ipfs

import (
	"encoding/hex"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDecodeCID(t *testing.T) {
	cid := "bafybeicn7i3soqdgr7dwnrwytgq4zxy7a5jpkizrvhm5mv6bgjd32wm3q4" // welcome-to-IPFS.jpg
	want, err := hex.DecodeString("4DFA372740668FC766C6D899A1CCDF1F0752F52331A9D9D657C13247BD599B87")
	assert.NilError(t, err)
	sha, err := decodeCID(cid)
	assert.NilError(t, err)
	assert.DeepEqual(t, want, sha)
}
