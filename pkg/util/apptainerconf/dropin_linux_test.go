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
	require.NoError(t, os.WriteFile(base, []byte("max loop devices = 64\nmount home = no\nbind path = /base\nbind path =\nimage driver = example"), 0o644))

	main, err := Parse(base)
	require.NoError(t, err)
	require.Equal(t, []string{"/base"}, main.BindPath)
	config, err := ParseWithDropIns(base, false)
	require.NoError(t, err)
	require.Equal(t, main, config)

	dropIns := filepath.Join(dir, "conf.d")
	require.NoError(t, os.Mkdir(dropIns, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "90-local.conf"), []byte("max loop devices = 128\nbind path = /a, /b\nbind path =\nbind path = /c"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "10-site.conf"), []byte("max loop devices = 96\nbind path = /site\nmount tmp = no"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "99-local.conf.bak"), []byte("max loop devices = 256"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dropIns, "99-dir.conf"), 0o755))

	config, err = ParseWithDropIns(base, false)
	require.NoError(t, err)
	require.Equal(t, uint(128), config.MaxLoopDevices)
	require.Equal(t, []string{"/a", "/b", "/c"}, config.BindPath)
	require.Equal(t, "example", config.ImageDriver)
	require.False(t, config.MountHome)
	require.False(t, config.MountTmp)

	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "95-clear.conf"), []byte("bind path =\nimage driver = \nmax loop devices ="), 0o644))
	config, err = ParseWithDropIns(base, false)
	require.NoError(t, err)
	require.Empty(t, config.BindPath)
	require.Empty(t, config.ImageDriver)
	require.Equal(t, uint(256), config.MaxLoopDevices)

	config, err = Parse(base)
	require.NoError(t, err)
	require.Equal(t, main, config)
}

func TestParseWithDropInsOwnership(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	for _, tc := range []struct {
		name string
		path string
		mode os.FileMode
		uid  int
	}{
		{name: "protected"},
		{"non-root directory", "conf.d", 0o755, 1},
		{"group-writable directory", "conf.d", 0o775, 0},
		{"other-writable directory", "conf.d", 0o757, 0},
		{"non-root file", "conf.d/site.conf", 0o644, 1},
		{"group-writable file", "conf.d/site.conf", 0o664, 0},
		{"other-writable file", "conf.d/site.conf", 0o646, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			base := filepath.Join(dir, "apptainer.conf")
			require.NoError(t, os.WriteFile(base, nil, 0o644))
			require.NoError(t, os.Mkdir(filepath.Join(dir, "conf.d"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "conf.d/site.conf"), []byte("mount home = no"), 0o644))

			if tc.path == "" {
				config, err := ParseWithDropIns(base, true)
				require.NoError(t, err)
				require.False(t, config.MountHome)
				return
			}

			path := filepath.Join(dir, tc.path)
			require.NoError(t, os.Chown(path, tc.uid, -1))
			require.NoError(t, os.Chmod(path, tc.mode))
			_, err := ParseWithDropIns(base, true)
			require.ErrorContains(t, err, path+" must be owned by root and writable only by root")
		})
	}
}

func TestParseWithDropInsSymlinks(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "apptainer.conf")
	dropIns := filepath.Join(dir, "conf.d")
	target := filepath.Join(dir, "site.conf")
	require.NoError(t, os.WriteFile(base, nil, 0o644))
	require.NoError(t, os.WriteFile(target, []byte("mount home = no"), 0o644))
	require.NoError(t, os.Mkdir(dropIns, 0o755))
	require.NoError(t, os.Symlink(target, filepath.Join(dropIns, "site.conf")))

	config, err := ParseWithDropIns(base, false)
	require.NoError(t, err)
	require.True(t, config.MountHome)

	require.NoError(t, os.RemoveAll(dropIns))
	require.NoError(t, os.Symlink(dir, dropIns))
	_, err = ParseWithDropIns(base, false)
	require.Error(t, err)
}
