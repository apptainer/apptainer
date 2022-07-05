package run

import (
	"io/fs"
	"os"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
)

// issue409 - need to handle situations where the home directory is not accessible.
func (c ctx) issue409(t *testing.T) {
	tests := []struct {
		name     string
		filemode fs.FileMode
		exit     int
	}{
		{
			name:     "accessible home directory",
			filemode: 0o700,
			exit:     0,
		},
		{
			name:     "read only home directory",
			filemode: 0o400,
			exit:     0,
		},
		{
			name:     "inaccessible only home directory",
			filemode: 0o000,
			exit:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempHomeDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "", "")
			defer cleanup(t)

			err := os.Chmod(tempHomeDir, tt.filemode)
			if err != nil {
				t.Fatalf("failed to modify the temporary home directory: %s", err)
			}

			cmdArgs := []string{"oras://ghcr.io/apptainer/alpine:3.15.0", "/bin/true"}

			c.env.HomeDir = tempHomeDir
			c.env.RunApptainer(
				t,
				e2e.WithProfile(e2e.UserProfile),
				e2e.WithCommand("run"),
				e2e.WithArgs(cmdArgs...),
				e2e.ExpectExit(0),
			)
		})
	}
}
