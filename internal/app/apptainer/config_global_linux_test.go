// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"gotest.tools/v3/assert"
)

func TestGlobalConfigBindPathOptions(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "apptainer.conf")
	assert.NilError(t, os.WriteFile(configFile, []byte("bind path = /etc/hosts\n"), 0o644))

	err := GlobalConfig([]string{"bind path", "/a::dev,suid,/b:/c:dev,suid"}, configFile, false, GlobalConfigSet)
	assert.NilError(t, err)
	c, err := apptainerconf.Parse(configFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, c.BindPath, []string{"/a::dev,suid", "/b:/c:dev,suid", "/etc/hosts"})

	err = GlobalConfig([]string{"bind path", "/a::dev,suid"}, configFile, false, GlobalConfigUnset)
	assert.NilError(t, err)
	c, err = apptainerconf.Parse(configFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, c.BindPath, []string{"/b:/c:dev,suid", "/etc/hosts"})

	err = GlobalConfig([]string{"bind path", "/a::bogus"}, configFile, false, GlobalConfigSet)
	assert.ErrorContains(t, err, `unsupported mount option "bogus"`)
}

func TestGlobalConfigUnsupportedBindPathOption(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "apptainer.conf")
	assert.NilError(t, os.WriteFile(configFile, []byte("bind path = /data:/data:ro\n"), 0o644))

	// An existing entry with an unsupported option doesn't block other edits.
	err := GlobalConfig([]string{"bind path", "/a::dev"}, configFile, false, GlobalConfigSet)
	assert.NilError(t, err)
	c, err := apptainerconf.Parse(configFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, c.BindPath, []string{"/a::dev", "/data:/data:ro"})

	err = GlobalConfig([]string{"bind path", "/data:/data:ro"}, configFile, false, GlobalConfigUnset)
	assert.NilError(t, err)
	c, err = apptainerconf.Parse(configFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, c.BindPath, []string{"/a::dev"})

	assert.NilError(t, os.WriteFile(configFile, []byte("bind path = /data:/data:ro\n"), 0o644))
	err = GlobalConfig([]string{"bind path"}, configFile, false, GlobalConfigReset)
	assert.NilError(t, err)
	c, err = apptainerconf.Parse(configFile)
	assert.NilError(t, err)
	assert.Equal(t, len(c.BindPath), 0)
}
