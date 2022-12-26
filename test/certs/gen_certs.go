// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package main

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

var start = time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC)

// privateKeyFromPEM reads a private key from a PEM-encoded file in the keys corpus.
func privateKeyFromPEM(name string) (crypto.PrivateKey, error) {
	b, err := os.ReadFile(filepath.Join("..", "keys", name))
	if err != nil {
		return nil, err
	}

	return cryptoutils.UnmarshalPEMToPrivateKey(b, cryptoutils.SkipPassword)
}

// zeroReader is an io.Reader that always returns zeros, similar to /dev/zero.
type zeroReader struct{}

func (zeroReader) Read(b []byte) (n int, err error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

// createCertificate creates a new X.509 certificate.
func createCertificate(tmpl, parent *x509.Certificate, pub, pri any) (*x509.Certificate, error) {
	// Use predictable source of "randomness" to generate corpus deterministically.
	var rand zeroReader

	der, err := x509.CreateCertificate(rand, tmpl, parent, pub, pri)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(der)
}

// createRoot creates a self-signed root certificate.
func createRoot(start time.Time) (crypto.PrivateKey, *x509.Certificate, error) {
	key, err := privateKeyFromPEM("ed25519-private.pem")
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Apptainer"},
			CommonName:   "root",
		},
		NotBefore:             start,
		NotAfter:              start.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
		OCSPServer:            []string{"http://localhost:9999"},
	}

	c, err := createCertificate(tmpl, tmpl, key.(crypto.Signer).Public(), key)
	return key, c, err
}

// createIntermediate creates an intermediate certificate using the supplied parent key/cert.
func createIntermediate(start time.Time, parentKey crypto.PrivateKey, parent *x509.Certificate) (crypto.PrivateKey, *x509.Certificate, error) {
	key, err := privateKeyFromPEM("ecdsa-private.pem")
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Apptainer"},
			CommonName:   "intermediate",
		},
		NotBefore:             start,
		NotAfter:              start.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		OCSPServer:            []string{"http://localhost:9999"},
	}

	c, err := createCertificate(tmpl, parent, key.(crypto.Signer).Public(), parentKey)
	return key, c, err
}

// createIntermediate creates a leaf certificate using the supplied parent key/cert.
func createLeaf(start time.Time, parentKey crypto.PrivateKey, parent *x509.Certificate) (*x509.Certificate, error) {
	key, err := privateKeyFromPEM("rsa-private.pem")
	if err != nil {
		return nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Apptainer"},
			CommonName:   "leaf",
		},
		NotBefore: start,
		NotAfter:  start.AddDate(10, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageCodeSigning,
		},
		MaxPathLenZero: true,
		OCSPServer:     []string{"http://localhost:9999"},
	}

	return createCertificate(tmpl, parent, key.(crypto.Signer).Public(), parentKey)
}

// writeCerts generates certificates and writes them to disk.
func writeCerts() error {
	pri, root, err := createRoot(start)
	if err != nil {
		return err
	}

	pri, intermediate, err := createIntermediate(start, pri, root)
	if err != nil {
		return err
	}

	leaf, err := createLeaf(start, pri, intermediate)
	if err != nil {
		return err
	}

	outputs := []struct {
		cert *x509.Certificate
		path string
	}{
		{
			cert: root,
			path: "root.pem",
		},
		{
			cert: intermediate,
			path: "intermediate.pem",
		},
		{
			cert: leaf,
			path: "leaf.pem",
		},
	}

	for _, output := range outputs {
		b, err := cryptoutils.MarshalCertificateToPEM(output.cert)
		if err != nil {
			return err
		}

		if err := os.WriteFile(output.path, b, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := writeCerts(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
