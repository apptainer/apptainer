// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package apptainer

import (
	"context"

	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/sigstore/sigstore/pkg/signature"
)

type signer struct {
	opts []integrity.SignerOpt
}

// SignOpt are used to configure s.
type SignOpt func(s *signer) error

// OptSignWithSigner specifies ss be used to generate signature(s).
func OptSignWithSigner(ss signature.Signer) SignOpt {
	return func(s *signer) error {
		s.opts = append(s.opts, integrity.OptSignWithSigner(ss))
		return nil
	}
}

// OptSignEntitySelector specifies f be used to select (and decrypt, if necessary) the entity to
// use to generate signature(s).
func OptSignEntitySelector(f sypgp.EntitySelector) SignOpt {
	return func(s *signer) error {
		e, err := sypgp.GetPrivateEntity(f)
		if err != nil {
			return err
		}

		s.opts = append(s.opts, integrity.OptSignWithEntity(e))

		return nil
	}
}

// OptSignGroup specifies that a signature be applied to cover all objects in the group with the
// specified groupID. This may be called multiple times to add multiple group signatures.
func OptSignGroup(groupID uint32) SignOpt {
	return func(s *signer) error {
		s.opts = append(s.opts, integrity.OptSignGroup(groupID))
		return nil
	}
}

// OptSignObjects specifies that one or more signature(s) be applied to cover objects with the
// specified ids. One signature will be applied for each group ID associated with the object(s).
// This may be called multiple times to add multiple signatures.
func OptSignObjects(ids ...uint32) SignOpt {
	return func(s *signer) error {
		s.opts = append(s.opts, integrity.OptSignObjects(ids...))
		return nil
	}
}

// Sign adds one or more digital signatures to the SIF image found at path, according to opts. Key
// material must be provided via OptSignEntitySelector.
//
// By default, one digital signature is added per object group in f. To override this behavior,
// consider using OptSignGroup and/or OptSignObject.
func Sign(ctx context.Context, path string, opts ...SignOpt) error {
	// Apply options to signer.
	s := signer{
		opts: []integrity.SignerOpt{
			integrity.OptSignWithContext(ctx),
		},
	}
	for _, opt := range opts {
		if err := opt(&s); err != nil {
			return err
		}
	}

	// Load container.
	f, err := sif.LoadContainerFromPath(path)
	if err != nil {
		return err
	}
	defer f.UnloadContainer()

	// Apply signature(s).
	is, err := integrity.NewSigner(f, s.opts...)
	if err != nil {
		return err
	}
	return is.Sign()
}
