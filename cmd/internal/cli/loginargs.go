// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"io"
	"os"
	"strings"

	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/sylog"
)

func ObtainLoginArgs(name string) *apptainer.LoginArgs {
	var loginArgs apptainer.LoginArgs

	loginArgs.Name = name

	loginArgs.Username = loginUsername
	loginArgs.Password = loginPassword
	loginArgs.Tokenfile = loginTokenFile
	loginArgs.Insecure = loginInsecure
	loginArgs.ReqAuthFile = reqAuthFile

	if loginPasswordStdin {
		p, err := io.ReadAll(os.Stdin)
		if err != nil {
			sylog.Fatalf("Failed to read password from stdin: %s", err)
		}
		loginArgs.Password = strings.TrimSuffix(string(p), "\n")
		loginArgs.Password = strings.TrimSuffix(loginArgs.Password, "\r")
	}

	return &loginArgs
}
