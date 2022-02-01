// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"github.com/apptainer/apptainer/cmd/internal/cli"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

func main() {
	useragent.InitValue(buildcfg.PACKAGE_NAME, buildcfg.PACKAGE_VERSION)

	// In cmd/internal/cli/apptainer.go
	cli.ExecuteApptainer()
}
