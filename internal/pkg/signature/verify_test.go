// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package signature

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/apptainer/container-key-client/client"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	testFingerPrint    = "F34371D0ACD5D09EB9BD853A80600A5FA11BBD29"
	invalidFingerPrint = "0000000000000000000000000000000000000000"
)

// getTestVerifier returns a fixed test Verifier.
func getTestVerifier(t *testing.T, file string) signature.Verifier {
	t.Helper()

	path := filepath.Join("..", "..", "..", "test", "keys", file)

	sv, err := signature.LoadVerifierFromPEMFile(path, crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	return sv
}

// getCertificate returns the certificate read from the specified file.
func getCertificate(t *testing.T, file string) *x509.Certificate {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "certs", file))
	if err != nil {
		t.Fatal(err)
	}

	p, _ := pem.Decode(b)
	if p == nil {
		t.Fatal("failed to decode PEM")
	}

	c, err := x509.ParseCertificate(p.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	return c
}

// getCertificatePool returns a pool of certificates read from the specified file.
func getCertificatePool(t *testing.T, file string) *x509.CertPool {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "certs", file))
	if err != nil {
		t.Fatal(err)
	}

	pool := x509.NewCertPool()

	for rest := bytes.TrimSpace(b); len(rest) > 0; {
		var p *pem.Block

		if p, rest = pem.Decode(rest); p == nil {
			t.Fatal("failed to decode PEM")
		}

		c, err := x509.ParseCertificate(p.Bytes)
		if err != nil {
			t.Fatal(err)
		}

		pool.AddCert(c)
	}

	return pool
}

type mockHKP struct {
	e *openpgp.Entity
}

func (m mockHKP) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/pgp-keys")

	wr, err := armor.Encode(w, openpgp.PublicKeyType, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer wr.Close()

	if err := m.e.Serialize(wr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func Test_newVerifier(t *testing.T) {
	cert := getCertificate(t, "leaf.pem")
	intermediates := getCertificatePool(t, "intermediate.pem")
	roots := getCertificatePool(t, "root.pem")

	sv := getTestVerifier(t, "ed25519-public.pem")

	pgpOpts := []client.Option{client.OptBearerToken("token")}

	tests := []struct {
		name         string
		opts         []VerifyOpt
		wantErr      error
		wantVerifier verifier
	}{
		{
			name:         "Defaults",
			wantVerifier: verifier{},
		},
		{
			name: "OptVerifyWithCertificate",
			opts: []VerifyOpt{
				OptVerifyWithCertificate(cert),
				OptVerifyWithIntermediates(intermediates),
				OptVerifyWithRoots(roots),
			},
			wantVerifier: verifier{
				certs:         []*x509.Certificate{cert},
				intermediates: intermediates,
				roots:         roots,
			},
		},
		{
			name:         "OptVerifyWithVerifier",
			opts:         []VerifyOpt{OptVerifyWithVerifier(sv)},
			wantVerifier: verifier{svs: []signature.Verifier{sv}},
		},
		{
			name:         "OptVerifyWithPGP",
			opts:         []VerifyOpt{OptVerifyWithPGP()},
			wantVerifier: verifier{pgp: true},
		},
		{
			name: "OptVerifyWithPGPOpts",
			opts: []VerifyOpt{OptVerifyWithPGP(pgpOpts...)},
			wantVerifier: verifier{
				pgp:     true,
				pgpOpts: pgpOpts,
			},
		},
		{
			name:         "OptVerifyGroup",
			opts:         []VerifyOpt{OptVerifyGroup(1)},
			wantVerifier: verifier{groupIDs: []uint32{1}},
		},
		{
			name:         "OptVerifyObject",
			opts:         []VerifyOpt{OptVerifyObject(1)},
			wantVerifier: verifier{objectIDs: []uint32{1}},
		},
		{
			name:         "OptVerifyAll",
			opts:         []VerifyOpt{OptVerifyAll()},
			wantVerifier: verifier{all: true},
		},
		{
			name:         "OptVerifyLegacy",
			opts:         []VerifyOpt{OptVerifyLegacy()},
			wantVerifier: verifier{legacy: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := newVerifier(tt.opts)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}

			if got, want := v, tt.wantVerifier; !reflect.DeepEqual(got, want) {
				t.Errorf("got verifier %v, want %v", got, want)
			}
		})
	}
}

func Test_verifier_getOpts(t *testing.T) {
	cert := getCertificate(t, "leaf.pem")
	intermediates := getCertificatePool(t, "intermediate.pem")
	roots := getCertificatePool(t, "root.pem")

	emptyImage, err := sif.LoadContainerFromPath(filepath.Join("..", "..", "..", "test", "images", "empty.sif"),
		sif.OptLoadWithFlag(os.O_RDONLY),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer emptyImage.UnloadContainer()

	oneGroupImage, err := sif.LoadContainerFromPath(filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
		sif.OptLoadWithFlag(os.O_RDONLY),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer oneGroupImage.UnloadContainer()

	cb := func(*sif.FileImage, integrity.VerifyResult) bool { return false }

	tests := []struct {
		name     string
		v        verifier
		f        *sif.FileImage
		wantErr  error
		wantOpts int
	}{
		{
			name: "TLSRequired",
			f:    emptyImage,
			v: verifier{
				pgp: true,
				pgpOpts: []client.Option{
					client.OptBaseURL("hkp://pool.sks-keyservers.net"),
					client.OptBearerToken("blah"),
				},
			},
			wantErr: client.ErrTLSRequired,
		},
		{
			name:    "NoObjects",
			f:       emptyImage,
			v:       verifier{legacy: true},
			wantErr: sif.ErrNoObjects,
		},
		{
			name: "Certificate",
			v: verifier{
				certs:         []*x509.Certificate{cert},
				intermediates: intermediates,
				roots:         roots,
			},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name: "Verifier",
			v: verifier{
				svs: []signature.Verifier{
					getTestVerifier(t, "ed25519-public.pem"),
				},
			},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name: "PGP",
			v: verifier{
				pgp: true,
			},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name: "PGPOpts",
			v: verifier{
				pgp: true,
				pgpOpts: []client.Option{
					client.OptBearerToken("token"),
				},
			},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "Group1",
			v:        verifier{groupIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "Object1",
			v:        verifier{objectIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "All",
			v:        verifier{all: true},
			f:        oneGroupImage,
			wantOpts: 1,
		},
		{
			name:     "Legacy",
			v:        verifier{legacy: true},
			f:        oneGroupImage,
			wantOpts: 3,
		},
		{
			name:     "LegacyGroup1",
			v:        verifier{legacy: true, groupIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 3,
		},
		{
			name:     "LegacyObject1",
			v:        verifier{legacy: true, objectIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 3,
		},
		{
			name:     "LegacyAll",
			v:        verifier{legacy: true, all: true},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "Callback",
			v:        verifier{cb: cb},
			f:        oneGroupImage,
			wantOpts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := tt.v.getOpts(context.Background(), tt.f)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}

			if got, want := len(opts), tt.wantOpts; got != want {
				t.Errorf("got %v options, want %v", got, want)
			}
		})
	}
}

func TestVerify(t *testing.T) { //nolint:maintidx
	cert := getCertificate(t, "leaf.pem")
	intermediates := getCertificatePool(t, "intermediate.pem")
	roots := getCertificatePool(t, "root.pem")

	ed25519 := getTestVerifier(t, "ed25519-public.pem")
	ed25519Pub, err := ed25519.PublicKey()
	if err != nil {
		t.Fatal(err)
	}

	rsa := getTestVerifier(t, "rsa-public.pem")
	rsaPub, err := rsa.PublicKey()
	if err != nil {
		t.Fatal(err)
	}

	// Start up a mock HKP server.
	e := getTestEntity(t)
	s := httptest.NewServer(mockHKP{e: e})
	defer s.Close()

	tests := []struct {
		name           string
		path           string
		opts           []VerifyOpt
		wantVerified   [][]uint32
		wantEntity     *openpgp.Entity
		wantPublicKeys []crypto.PublicKey
		wantErr        error
	}{
		{
			name:    "SignatureNotFound",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundDSSE",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group-signed-dsse.sif"),
			opts:    []VerifyOpt{OptVerifyLegacy()},
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundPGP",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			opts:    []VerifyOpt{OptVerifyLegacy()},
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundLegacy",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundLegacyAll",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-all.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundLegacyGroup",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-group.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name: "Certificate",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithCertificate(cert),
				OptVerifyWithIntermediates(intermediates),
				OptVerifyWithRoots(roots),
			},
			wantVerified:   [][]uint32{{1, 2}},
			wantPublicKeys: []crypto.PublicKey{rsaPub},
		},
		{
			name: "VerifierEd25519",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(ed25519),
			},
			wantVerified:   [][]uint32{{1, 2}},
			wantPublicKeys: []crypto.PublicKey{ed25519Pub},
		},
		{
			name: "VerifierRSA",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(rsa),
			},
			wantVerified:   [][]uint32{{1, 2}},
			wantPublicKeys: []crypto.PublicKey{rsaPub},
		},
		{
			name: "PGP",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name: "OptVerifyGroupVerifier",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(ed25519),
				OptVerifyGroup(1),
			},
			wantVerified:   [][]uint32{{1, 2}},
			wantPublicKeys: []crypto.PublicKey{ed25519Pub},
		},
		{
			name: "OptVerifyGroupPGP",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyGroup(1),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name: "OptVerifyObjectVerifier",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(ed25519),
				OptVerifyObject(1),
			},
			wantVerified:   [][]uint32{{1}},
			wantPublicKeys: []crypto.PublicKey{ed25519Pub},
		},
		{
			name: "OptVerifyObjectPGP",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyObject(1),
			},
			wantVerified: [][]uint32{{1}},
			wantEntity:   e,
		},
		{
			name: "Legacy",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
			},
			wantVerified: [][]uint32{{2}},
			wantEntity:   e,
		},
		{
			name: "LegacyOptVerifyObject",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-all.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
				OptVerifyObject(1),
			},
			wantVerified: [][]uint32{{1}},
			wantEntity:   e,
		},
		{
			name: "LegacyOptVerifyAll",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-all.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
				OptVerifyAll(),
			},
			wantVerified: [][]uint32{{1}, {2}},
			wantEntity:   e,
		},
		{
			name: "LegacyOptVerifyGroup",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-group.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
				OptVerifyGroup(1),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0

			cb := func(_ *sif.FileImage, r integrity.VerifyResult) bool {
				defer func() { i++ }()

				if i >= len(tt.wantVerified) {
					t.Fatalf("wantVerified consumed")
				}

				if got, want := r.Verified(), tt.wantVerified[i]; len(got) != len(want) {
					t.Errorf("got %v verified, want %v", got, want)
				} else {
					for i, od := range got {
						if got, want := od.ID(), want[i]; got != want {
							t.Errorf("got ID %v, want %v", got, want)
						}
					}
				}

				if tt.wantEntity != nil {
					if got, want := r.Entity().PrimaryKey, tt.wantEntity.PrimaryKey; !reflect.DeepEqual(got, want) {
						t.Errorf("got entity public key %+v, want %+v", got, want)
					}
				}

				if tt.wantPublicKeys != nil {
					if got, want := r.Keys(), tt.wantPublicKeys; !reflect.DeepEqual(got, want) {
						t.Errorf("got public keys %+v, want %+v", got, want)
					}
				}

				if got, want := r.Error(), tt.wantErr; !errors.Is(got, want) {
					t.Errorf("got error %v, want %v", got, want)
				}

				return false
			}
			tt.opts = append(tt.opts, OptVerifyCallback(cb))

			err := Verify(context.Background(), tt.path, tt.opts...)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}
		})
	}
}

func TestVerifyFingerPrint(t *testing.T) {
	// Start up a mock HKP server.
	e := getTestEntity(t)
	s := httptest.NewServer(mockHKP{e: e})
	defer s.Close()

	tests := []struct {
		name         string
		path         string
		fingerprints []string
		opts         []VerifyOpt
		wantVerified [][]uint32
		wantEntity   *openpgp.Entity
		wantErr      error
	}{
		{
			name:         "SignatureNotFound",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundNonLegacy",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint},
			opts:         []VerifyOpt{OptVerifyLegacy()},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundLegacy",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundLegacyAll",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-all.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundLegacyGroup",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-group.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "PGP",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name:         "OptVerifyGroupPGP",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyGroup(1),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name:         "OptVerifyObjectPGP",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyObject(1),
			},
			wantVerified: [][]uint32{{1}},
			wantEntity:   e,
		},
		{
			name:         "Legacy",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
			},
			wantVerified: [][]uint32{{2}},
			wantEntity:   e,
		},
		{
			name:         "LegacyOptVerifyObject",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-all.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
				OptVerifyObject(1),
			},
			wantVerified: [][]uint32{{1}},
			wantEntity:   e,
		},
		{
			name:         "LegacyOptVerifyAll",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-all.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
				OptVerifyAll(),
			},
			wantVerified: [][]uint32{{1}, {2}},
			wantEntity:   e,
		},
		{
			name:         "LegacyOptVerifyGroup",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-legacy-group.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
				OptVerifyGroup(1),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name:         "SingleFingerprintWrong",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{invalidFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
			wantErr:      errNotSignedByRequired,
		},
		{
			name:         "MultipleFingerprintOneWrong",
			path:         filepath.Join("..", "..", "..", "test", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint, invalidFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
			wantErr:      errNotSignedByRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0

			cb := func(_ *sif.FileImage, r integrity.VerifyResult) bool {
				defer func() { i++ }()

				if i >= len(tt.wantVerified) {
					t.Fatalf("wantVerified consumed")
				}

				if got, want := r.Verified(), tt.wantVerified[i]; len(got) != len(want) {
					t.Errorf("got %v verified, want %v", got, want)
				} else {
					for i, od := range got {
						if got, want := od.ID(), want[i]; got != want {
							t.Errorf("got ID %v, want %v", got, want)
						}
					}
				}

				if got, want := r.Entity().PrimaryKey, tt.wantEntity.PrimaryKey; !reflect.DeepEqual(got, want) {
					t.Errorf("got entity public key %+v, want %+v", got, want)
				}

				return false
			}
			tt.opts = append(tt.opts, OptVerifyCallback(cb))
			err := VerifyFingerprints(context.Background(), tt.path, tt.fingerprints, tt.opts...)
			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}
		})
	}
}
