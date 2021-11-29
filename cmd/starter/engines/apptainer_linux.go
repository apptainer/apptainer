// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build apptainer_engine
// +build apptainer_engine

package engines

import (
	// register the apptainer runtime engine
	_ "github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer"
)
