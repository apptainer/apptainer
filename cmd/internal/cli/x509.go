// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

var errFailedToDecodePEM = errors.New("failed to decode PEM")

// loadCertificate returns the certificate read from path.
func loadCertificate(path string) (*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	p, _ := pem.Decode(b)
	if p == nil {
		return nil, errFailedToDecodePEM
	}

	return x509.ParseCertificate(p.Bytes)
}

// loadCertificatePool returns the pool of certificates read from path.
func loadCertificatePool(path string) (*x509.CertPool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()

	for rest := bytes.TrimSpace(b); len(rest) > 0; {
		var p *pem.Block

		if p, rest = pem.Decode(rest); p == nil {
			return nil, errFailedToDecodePEM
		}

		c, err := x509.ParseCertificate(p.Bytes)
		if err != nil {
			return nil, err
		}

		pool.AddCert(c)
	}

	return pool, nil
}
