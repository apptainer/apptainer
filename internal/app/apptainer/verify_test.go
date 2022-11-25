// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package apptainer

import (
	"context"
	"crypto"
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
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	testFingerPrint    = "12045C8C0B1004D058DE4BEDA20C27EE7FF7BA84"
	invalidFingerPrint = "0000000000000000000000000000000000000000"
)

// getTestSignerVerifier returns a fixed test SignerVerifier.
func getTestSignerVerifier(t *testing.T) signature.SignerVerifier {
	path := filepath.Join("..", "..", "..", "test", "keys", "private.pem")

	sv, err := signature.LoadSignerVerifierFromPEMFile(path, crypto.SHA256, cryptoutils.SkipPassword)
	if err != nil {
		t.Fatal(err)
	}

	return sv
}

// getTestEntity returns a fixed test PGP entity.
func getTestEntity(t *testing.T) *openpgp.Entity {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "..", "test", "keys", "private.asc"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	el, err := openpgp.ReadArmoredKeyRing(f)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(el), 1; got != want {
		t.Fatalf("got %v entities, want %v", got, want)
	}
	return el[0]
}

type mockHKP struct {
	e *openpgp.Entity
}

func (m mockHKP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	sv := getTestSignerVerifier(t)
	opts := []client.Option{client.OptBearerToken("token")}

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
			opts: []VerifyOpt{OptVerifyWithPGP(opts...)},
			wantVerifier: verifier{
				pgp:     true,
				pgpOpts: opts,
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
		tt := tt
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
	emptyImage, err := sif.LoadContainerFromPath(filepath.Join("testdata", "images", "empty.sif"),
		sif.OptLoadWithFlag(os.O_RDONLY),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer emptyImage.UnloadContainer()

	oneGroupImage, err := sif.LoadContainerFromPath(filepath.Join("testdata", "images", "one-group.sif"),
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
			name: "Verifier",
			v: verifier{
				svs: []signature.Verifier{
					getTestSignerVerifier(t),
				},
			},
			f:        oneGroupImage,
			wantOpts: 1,
		},
		{
			name: "PGP",
			v: verifier{
				pgp: true,
			},
			f:        oneGroupImage,
			wantOpts: 1,
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
			wantOpts: 1,
		},
		{
			name:     "Group1",
			v:        verifier{groupIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 1,
		},
		{
			name:     "Object1",
			v:        verifier{objectIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 1,
		},
		{
			name:     "All",
			v:        verifier{all: true},
			f:        oneGroupImage,
			wantOpts: 0,
		},
		{
			name:     "Legacy",
			v:        verifier{legacy: true},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "LegacyGroup1",
			v:        verifier{legacy: true, groupIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "LegacyObject1",
			v:        verifier{legacy: true, objectIDs: []uint32{1}},
			f:        oneGroupImage,
			wantOpts: 2,
		},
		{
			name:     "LegacyAll",
			v:        verifier{legacy: true, all: true},
			f:        oneGroupImage,
			wantOpts: 1,
		},
		{
			name:     "Callback",
			v:        verifier{cb: cb},
			f:        oneGroupImage,
			wantOpts: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
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

func TestVerify(t *testing.T) {
	sv := getTestSignerVerifier(t)
	pub, err := sv.PublicKey()
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
			path:    filepath.Join("testdata", "images", "one-group.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundDSSE",
			path:    filepath.Join("testdata", "images", "one-group-signed-dsse.sif"),
			opts:    []VerifyOpt{OptVerifyLegacy()},
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundPGP",
			path:    filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
			opts:    []VerifyOpt{OptVerifyLegacy()},
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundLegacy",
			path:    filepath.Join("testdata", "images", "one-group-signed-legacy.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundLegacyAll",
			path:    filepath.Join("testdata", "images", "one-group-signed-legacy-all.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name:    "SignatureNotFoundLegacyGroup",
			path:    filepath.Join("testdata", "images", "one-group-signed-legacy-group.sif"),
			wantErr: &integrity.SignatureNotFoundError{},
		},
		{
			name: "Verifier",
			path: filepath.Join("testdata", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(sv),
			},
			wantVerified:   [][]uint32{{1, 2}},
			wantPublicKeys: []crypto.PublicKey{pub},
		},
		{
			name: "PGP",
			path: filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name: "OptVerifyGroupVerifier",
			path: filepath.Join("testdata", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(sv),
				OptVerifyGroup(1),
			},
			wantVerified:   [][]uint32{{1, 2}},
			wantPublicKeys: []crypto.PublicKey{pub},
		},
		{
			name: "OptVerifyGroupPGP",
			path: filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyGroup(1),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name: "OptVerifyObjectVerifier",
			path: filepath.Join("testdata", "images", "one-group-signed-dsse.sif"),
			opts: []VerifyOpt{
				OptVerifyWithVerifier(sv),
				OptVerifyObject(1),
			},
			wantVerified:   [][]uint32{{1}},
			wantPublicKeys: []crypto.PublicKey{pub},
		},
		{
			name: "OptVerifyObjectPGP",
			path: filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyObject(1),
			},
			wantVerified: [][]uint32{{1}},
			wantEntity:   e,
		},
		{
			name: "Legacy",
			path: filepath.Join("testdata", "images", "one-group-signed-legacy.sif"),
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
				OptVerifyLegacy(),
			},
			wantVerified: [][]uint32{{2}},
			wantEntity:   e,
		},
		{
			name: "LegacyOptVerifyObject",
			path: filepath.Join("testdata", "images", "one-group-signed-legacy-all.sif"),
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
			path: filepath.Join("testdata", "images", "one-group-signed-legacy-all.sif"),
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
			path: filepath.Join("testdata", "images", "one-group-signed-legacy-group.sif"),
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			i := 0

			cb := func(f *sif.FileImage, r integrity.VerifyResult) bool {
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
			path:         filepath.Join("testdata", "images", "one-group.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundNonLegacy",
			path:         filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint},
			opts:         []VerifyOpt{OptVerifyLegacy()},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundLegacy",
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundLegacyAll",
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy-all.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "SignatureNotFoundLegacyGroup",
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy-group.sif"),
			fingerprints: []string{testFingerPrint},
			wantErr:      &integrity.SignatureNotFoundError{},
		},
		{
			name:         "PGP",
			path:         filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
			fingerprints: []string{testFingerPrint},
			opts: []VerifyOpt{
				OptVerifyWithPGP(client.OptBaseURL(s.URL)),
			},
			wantVerified: [][]uint32{{1, 2}},
			wantEntity:   e,
		},
		{
			name:         "OptVerifyGroupPGP",
			path:         filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy-all.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy-all.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-legacy-group.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
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
			path:         filepath.Join("testdata", "images", "one-group-signed-pgp.sif"),
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			i := 0

			cb := func(f *sif.FileImage, r integrity.VerifyResult) bool {
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
