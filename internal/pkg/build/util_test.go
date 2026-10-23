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

	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/stretchr/testify/require"
)

// TestCreateStageFile covers missing host files and preserves staging errors.
func TestCreateStageFile(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		missingDest bool
		missingTemp bool
		wantContent string
		wantError   string
	}{
		{name: "present", source: "file", wantContent: "127.0.0.1 localhost\n\n"},
		{name: "source symlink", source: "symlink", wantContent: "127.0.0.1 localhost\n\n"},
		{name: "empty", source: "empty", wantContent: "\n"},
		{name: "missing source", source: "missing"},
		{name: "dangling source symlink", source: "dangling"},
		{name: "missing destination", source: "file", missingDest: true},
		{name: "source read error", source: "directory", wantError: "failed to read"},
		{name: "staging creation error", source: "file", missingTemp: true, wantError: "failed to create staging"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "host", "hosts")
			require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
			switch tt.source {
			case "file":
				require.NoError(t, os.WriteFile(source, []byte("127.0.0.1 localhost\n"), 0o644))
			case "empty":
				require.NoError(t, os.WriteFile(source, nil, 0o644))
			case "symlink":
				require.NoError(t, os.WriteFile(source+".target", []byte("127.0.0.1 localhost\n"), 0o644))
				require.NoError(t, os.Symlink(filepath.Base(source)+".target", source))
			case "dangling":
				require.NoError(t, os.Symlink("missing", source))
			case "directory":
				require.NoError(t, os.Mkdir(source, 0o755))
			}

			b := &types.Bundle{
				RootfsPath: filepath.Join(dir, "rootfs"),
				TmpDir:     filepath.Join(dir, "stage"),
			}
			if !tt.missingDest {
				dest := filepath.Join(b.RootfsPath, source)
				require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
				require.NoError(t, os.WriteFile(dest, []byte("container hosts\n"), 0o644))
			}
			if !tt.missingTemp {
				require.NoError(t, os.Mkdir(b.TmpDir, 0o755))
			}

			staged, err := createStageFile(source, b, "Host resolution could fail")
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Empty(t, staged)
				return
			}
			require.NoError(t, err)
			if tt.wantContent == "" {
				require.Empty(t, staged)
				require.NoFileExists(t, filepath.Join(b.TmpDir, filepath.Base(source)))
				return
			}
			require.Equal(t, filepath.Join(b.TmpDir, filepath.Base(source)), staged)
			content, err := os.ReadFile(staged)
			require.NoError(t, err)
			require.Equal(t, tt.wantContent, string(content))
		})
	}
}
