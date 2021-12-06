// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"

	"github.com/apptainer/apptainer/internal/app/singularity"
	"github.com/apptainer/apptainer/pkg/sylog"
	scskeyclient "github.com/sylabs/scs-key-client/client"
)

// checkSIFFingerprint checks whether a bootstrap SIF image verifies, and was signed with a specified fingerprint
func checkSIFFingerprint(ctx context.Context, imagePath string, fingerprints []string, co ...scskeyclient.Option) error {
	sylog.Infof("Checking bootstrap image verifies with fingerprint(s): %v", fingerprints)
	return singularity.VerifyFingerprints(ctx, imagePath, fingerprints, singularity.OptVerifyUseKeyServer(co...))
}

// verifySIF checks whether a bootstrap SIF image verifies
func verifySIF(ctx context.Context, imagePath string, co ...scskeyclient.Option) error {
	sylog.Infof("Verifying bootstrap image %s", imagePath)
	return singularity.Verify(ctx, imagePath, singularity.OptVerifyUseKeyServer(co...))
}
