// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package build

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestHashBaseImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "base.sif")
	mustWriteFile(t, path, "some sif content")

	hash, err := hashBaseImage(path)
	assert.NilError(t, err)
	assert.Equal(t, hash, "sha256:e2d30bf8186c9e6a2506891ec08a67d730700522756ef5cce23f00887402985b")

	_, err = hashBaseImage(dir)
	assert.ErrorContains(t, err, "does not support a sandbox base image")
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o644))
}
