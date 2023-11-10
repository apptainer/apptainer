// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package signature

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/sypgp"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/container-key-client/client"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/signature"
)

// TODO - error overlaps with ECL - should probably become part of a common errors package at some point.
var errNotSignedByRequired = errors.New("image not signed by required entities")

type VerifyCallback func(*sif.FileImage, integrity.VerifyResult) bool

type verifier struct {
	certs         []*x509.Certificate
	intermediates *x509.CertPool
	roots         *x509.CertPool
	ocsp          bool
	svs           []signature.Verifier
	pgp           bool
	pgpOpts       []client.Option
	groupIDs      []uint32
	objectIDs     []uint32
	all           bool
	legacy        bool
	cb            VerifyCallback
}

// VerifyOpt are used to configure v.
type VerifyOpt func(v *verifier) error

// OptVerifyWithCertificate appends c as a source of key material to verify signatures.
func OptVerifyWithCertificate(c *x509.Certificate) VerifyOpt {
	return func(v *verifier) error {
		v.certs = append(v.certs, c)
		return nil
	}
}

// OptVerifyWithIntermediates specifies p as the pool of certificates that can be used to form a
// chain from the leaf certificate to a root certificate.
func OptVerifyWithIntermediates(p *x509.CertPool) VerifyOpt {
	return func(v *verifier) error {
		v.intermediates = p
		return nil
	}
}

// OptVerifyWithRoots specifies p as the pool of root certificates to use, instead of the system
// roots or the platform verifier.
func OptVerifyWithRoots(p *x509.CertPool) VerifyOpt {
	return func(v *verifier) error {
		v.roots = p
		return nil
	}
}

// OptVerifyWithVerifier appends sv as a source of key material to verify signatures.
func OptVerifyWithVerifier(sv signature.Verifier) VerifyOpt {
	return func(v *verifier) error {
		v.svs = append(v.svs, sv)
		return nil
	}
}

// OptVerifyWithPGP adds the local public keyring as a source of key material to verify signatures.
// If supplied, opts specify a keyserver to use in addition to the local public keyring.
func OptVerifyWithPGP(opts ...client.Option) VerifyOpt {
	return func(v *verifier) error {
		v.pgp = true
		v.pgpOpts = opts
		return nil
	}
}

// OptVerifyWithOCSP subjects the x509 certificate chains to online revocation checks,
// before the leaf certificate is deemed as trusted for validating the signature.
func OptVerifyWithOCSP() VerifyOpt {
	return func(v *verifier) error {
		v.ocsp = true

		return nil
	}
}

// OptVerifyGroup adds a verification task for the group with the specified groupID. This may be
// called multiple times to request verification of more than one group.
func OptVerifyGroup(groupID uint32) VerifyOpt {
	return func(v *verifier) error {
		v.groupIDs = append(v.groupIDs, groupID)
		return nil
	}
}

// OptVerifyObject adds a verification task for the object with the specified id. This may be
// called multiple times to request verification of more than one object.
func OptVerifyObject(id uint32) VerifyOpt {
	return func(v *verifier) error {
		v.objectIDs = append(v.objectIDs, id)
		return nil
	}
}

// OptVerifyAll adds one verification task per non-signature object in the image when verification
// of legacy signatures is enabled. When verification of legacy signatures is disabled (the
// default), this option has no effect.
func OptVerifyAll() VerifyOpt {
	return func(v *verifier) error {
		v.all = true
		return nil
	}
}

// OptVerifyLegacy enables verification of legacy signatures.
func OptVerifyLegacy() VerifyOpt {
	return func(v *verifier) error {
		v.legacy = true
		return nil
	}
}

// OptVerifyCallback registers f as the verification callback.
func OptVerifyCallback(cb VerifyCallback) VerifyOpt {
	return func(v *verifier) error {
		v.cb = cb
		return nil
	}
}

// newVerifier constructs a new verifier based on opts.
func newVerifier(opts []VerifyOpt) (verifier, error) {
	v := verifier{}
	for _, opt := range opts {
		if err := opt(&v); err != nil {
			return verifier{}, err
		}
	}
	return v, nil
}

// verifyCertificate attempts to verify c is a valid code signing certificate by building one or
// more chains from c to a certificate in roots, using certificates in intermediates if needed.
// This function does not do any revocation checking.
func verifyCertificate(c *x509.Certificate, intermediates, roots *x509.CertPool) (chains [][]*x509.Certificate, err error) {
	opts := x509.VerifyOptions{
		Intermediates: intermediates,
		Roots:         roots,
		KeyUsages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageCodeSigning,
		},
	}

	return c.Verify(opts)
}

// getOpts returns integrity.VerifierOpt necessary to validate f.
func (v verifier) getOpts(ctx context.Context, f *sif.FileImage) ([]integrity.VerifierOpt, error) {
	iopts := []integrity.VerifierOpt{
		integrity.OptVerifyWithContext(ctx),
	}

	// Add key material from certificate(s).
	for _, c := range v.certs {
		// verify that the leaf certificate is not tampered and that is adequate for signing purposes.
		chain, err := verifyCertificate(c, v.intermediates, v.roots)
		if err != nil {
			return nil, err
		}

		// Verify that the certificate is issued by a trustworthy CA (i.e the certificate chain is not revoked or expired).
		if v.ocsp {
			if len(chain) != 1 {
				return nil, fmt.Errorf("unhandled OCSP condition, chain length %d != 1", len(chain))
			}

			ocspErr := OCSPVerify(chain[0]...)
			if ocspErr != nil {
				// TODO: We need to decide whether this should be strict or permissive.
				return nil, ocspErr
			}

			sylog.Debugf("OCSP validation has passed")
		}

		// verify the signature by using the certificate.
		sv, err := signature.LoadVerifier(c.PublicKey, crypto.SHA256)
		if err != nil {
			return nil, err
		}

		iopts = append(iopts, integrity.OptVerifyWithVerifier(sv))
	}

	// Add explicitly provided key material source(s).
	for _, sv := range v.svs {
		iopts = append(iopts, integrity.OptVerifyWithVerifier(sv))
	}

	// Add PGP key material, if applicable.
	if v.pgp {
		var kr openpgp.KeyRing
		if v.pgpOpts != nil {
			hkr, err := sypgp.NewHybridKeyRing(ctx, v.pgpOpts...)
			if err != nil {
				return nil, err
			}
			kr = hkr
		} else {
			pkr, err := sypgp.PublicKeyRing()
			if err != nil {
				return nil, err
			}
			kr = pkr
		}

		// wrap the global keyring around
		global := sypgp.NewHandle(buildcfg.APPTAINER_CONFDIR, sypgp.GlobalHandleOpt())
		gkr, err := global.LoadPubKeyring()
		if err != nil {
			return nil, err
		}
		kr = sypgp.NewMultiKeyRing(gkr, kr)

		iopts = append(iopts, integrity.OptVerifyWithKeyRing(kr))
	}

	// Add group IDs, if applicable.
	for _, groupID := range v.groupIDs {
		iopts = append(iopts, integrity.OptVerifyGroup(groupID))
	}

	// Add objectIDs, if applicable.
	for _, objectID := range v.objectIDs {
		iopts = append(iopts, integrity.OptVerifyObject(objectID))
	}

	// Set legacy options, if applicable.
	if v.legacy {
		if v.all {
			iopts = append(iopts, integrity.OptVerifyLegacyAll())
		} else {
			iopts = append(iopts, integrity.OptVerifyLegacy())

			// If no objects explicitly selected, select system partition.
			if len(v.groupIDs) == 0 && len(v.objectIDs) == 0 {
				od, err := f.GetDescriptor(sif.WithPartitionType(sif.PartPrimSys))
				if err != nil {
					return nil, err
				}
				iopts = append(iopts, integrity.OptVerifyObject(od.ID()))
			}
		}
	}

	// Add callback, if applicable.
	if v.cb != nil {
		fn := func(r integrity.VerifyResult) bool {
			return v.cb(f, r)
		}
		iopts = append(iopts, integrity.OptVerifyCallback(fn))
	}

	return iopts, nil
}

// Verify verifies digital signature(s) in the SIF image found at path, according to opts.
//
// To use key material from an x.509 certificate, use OptVerifyWithCertificate. The system roots or
// the platform verifier will be used to verify the certificate, unless OptVerifyWithIntermediates
// and/or OptVerifyWithRoots are specified.
//
// To use raw key material, use OptVerifyWithVerifier.
//
// To use PGP key material, use OptVerifyWithPGP.
//
// By default, non-legacy signatures for all object groups are verified. To override the default
// behavior, consider using OptVerifyGroup, OptVerifyObject, OptVerifyAll, and/or OptVerifyLegacy.
func Verify(ctx context.Context, path string, opts ...VerifyOpt) error {
	v, err := newVerifier(opts)
	if err != nil {
		return err
	}

	// Load container.
	f, err := sif.LoadContainerFromPath(path, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return err
	}
	defer f.UnloadContainer()

	// Get options to validate f.
	vopts, err := v.getOpts(ctx, f)
	if err != nil {
		return err
	}

	// Verify signature(s).
	iv, err := integrity.NewVerifier(f, vopts...)
	if err != nil {
		return err
	}

	return iv.Verify()
}

// VerifyFingerprints verifies an image and checks it was signed by *all* of the provided
// fingerprints.
//
// To use key material from an x.509 certificate, use OptVerifyWithCertificate. The system roots or
// the platform verifier will be used to verify the certificate, unless OptVerifyWithIntermediates
// and/or OptVerifyWithRoots are specified.
//
// To use raw key material, use OptVerifyWithVerifier.
//
// To use PGP key material, use OptVerifyWithPGP.
//
// By default, non-legacy signatures for all object groups are verified. To override the default
// behavior, consider using OptVerifyGroup, OptVerifyObject, OptVerifyAll, and/or OptVerifyLegacy.
func VerifyFingerprints(ctx context.Context, path string, fingerprints []string, opts ...VerifyOpt) error {
	v, err := newVerifier(opts)
	if err != nil {
		return err
	}

	// Load container.
	f, err := sif.LoadContainerFromPath(path, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return err
	}
	defer f.UnloadContainer()

	// Get options to validate f.
	vopts, err := v.getOpts(ctx, f)
	if err != nil {
		return err
	}

	// Verify signature(s).
	iv, err := integrity.NewVerifier(f, vopts...)
	if err != nil {
		return err
	}
	err = iv.Verify()
	if err != nil {
		return err
	}

	// get signing entities fingerprints that have signed all selected objects
	keyfps, err := iv.AllSignedBy()
	if err != nil {
		return err
	}
	// were the selected objects signed by the provided fingerprints?

	m := map[string]bool{}
	for _, v := range fingerprints {
		m[v] = false
		for _, u := range keyfps {
			if strings.EqualFold(v, hex.EncodeToString(u[:])) {
				m[v] = true
			}
		}
	}
	for _, v := range m {
		if !v {
			return errNotSignedByRequired
		}
	}
	return nil
}
