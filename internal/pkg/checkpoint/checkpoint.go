// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package checkpoint

import (
	"path/filepath"

	"github.com/apptainer/apptainer/pkg/syfs"
)

const (
	checkpointStatePath = "checkpoint"
)

func StatePath() string {
	return filepath.Join(syfs.ConfigDir(), checkpointStatePath)
}
