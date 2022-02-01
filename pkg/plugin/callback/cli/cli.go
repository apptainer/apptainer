// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the URIs of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
)

// Command callback allows to add/modify commands and/or flags.
// This callback is called in cmd/internal/cli/apptainer.go and
// allows plugins to inject/modify commands and/or flags to existing
// apptainer commands.
type Command func(*cmdline.CommandManager)

// ApptainerEngineConfig callback allows to manipulate Apptainer
// runtime engine configuration.
// This callback is called in cmd/internal/cli/actions_linux.go and
// allows plugins to modify/alter runtime engine configuration. This
// is the place to inject custom binds.
type ApptainerEngineConfig func(*config.Common)
