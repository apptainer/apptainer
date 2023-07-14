// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"

	"github.com/apptainer/apptainer/internal/pkg/signature"
	"github.com/apptainer/apptainer/pkg/sylog"
	keyClient "github.com/apptainer/container-key-client/client"
)

// checkSIFFingerprint checks whether a bootstrap SIF image verifies, and was signed with a specified fingerprint
func checkSIFFingerprint(ctx context.Context, imagePath string, fingerprints []string, co ...keyClient.Option) error {
	sylog.Infof("Checking bootstrap image verifies with fingerprint(s): %v", fingerprints)
	return signature.VerifyFingerprints(ctx, imagePath, fingerprints, signature.OptVerifyWithPGP(co...))
}

// verifySIF checks whether a bootstrap SIF image verifies
func verifySIF(ctx context.Context, imagePath string, co ...keyClient.Option) error {
	sylog.Infof("Verifying bootstrap image %s", imagePath)
	return signature.Verify(ctx, imagePath, signature.OptVerifyWithPGP(co...))
}
