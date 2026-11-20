// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainerconf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWithDropIns(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "apptainer.conf")
	require.NoError(t, os.WriteFile(base, []byte("max loop devices = 64\nmount home = no\nbind path = /base"), 0o644))

	main, err := Parse(base)
	require.NoError(t, err)
	config, err := ParseWithDropIns(base, false)
	require.NoError(t, err)
	require.Equal(t, main, config)

	dropIns := filepath.Join(dir, "conf.d")
	require.NoError(t, os.Mkdir(dropIns, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "90-local.conf"), []byte("max loop devices = 128\nbind path = /a, /b\nbind path = /c"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "10-site.conf"), []byte("max loop devices = 96\nbind path = /site\nmount tmp = no"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "99-local.conf.bak"), []byte("max loop devices = 256"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dropIns, "99-dir.conf"), 0o755))

	config, err = ParseWithDropIns(base, false)
	require.NoError(t, err)
	require.Equal(t, uint(128), config.MaxLoopDevices)
	require.Equal(t, []string{"/a", "/b", "/c"}, config.BindPath)
	require.False(t, config.MountHome)
	require.False(t, config.MountTmp)

	config, err = Parse(base)
	require.NoError(t, err)
	require.Equal(t, main, config)
}
