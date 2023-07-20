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
	"context"
	"crypto"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

// getTestSigner returns a fixed test Signer.
func getTestSigner(t *testing.T, file string) signature.Signer {
	t.Helper()

	path := filepath.Join("..", "..", "..", "test", "keys", file)

	sv, err := signature.LoadSignerFromPEMFile(path, crypto.SHA256, cryptoutils.SkipPassword)
	if err != nil {
		t.Fatal(err)
	}

	return sv
}

// getTestEntity returns a fixed test PGP entity.
func getTestEntity(t *testing.T) *openpgp.Entity {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "..", "test", "keys", "pgp-private.asc"))
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

// tempFileFrom copies the file at path to a temporary file, and returns a reference to it.
func tempFileFrom(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	tf, err := os.CreateTemp("", "*.sif")
	if err != nil {
		return "", err
	}
	defer tf.Close()

	if _, err := io.Copy(tf, f); err != nil {
		return "", err
	}

	return tf.Name(), nil
}

func mockEntitySelector(t *testing.T) sypgp.EntitySelector {
	e := getTestEntity(t)

	return func(openpgp.EntityList) (*openpgp.Entity, error) {
		return e, nil
	}
}

func TestSign(t *testing.T) {
	ecdsa := getTestSigner(t, "ecdsa-private.pem")
	ed25519 := getTestSigner(t, "ed25519-private.pem")
	rsa := getTestSigner(t, "rsa-private.pem")
	es := mockEntitySelector(t)

	tests := []struct {
		name    string
		path    string
		opts    []SignOpt
		wantErr error
	}{
		{
			name:    "ErrNoKeyMaterial",
			path:    filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			wantErr: integrity.ErrNoKeyMaterial,
		},
		{
			name: "OptSignWithSignerECDSA",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			opts: []SignOpt{OptSignWithSigner(ecdsa)},
		},
		{
			name: "OptSignWithSignerEd25519",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			opts: []SignOpt{OptSignWithSigner(ed25519)},
		},
		{
			name: "OptSignWithSignerRSA",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			opts: []SignOpt{OptSignWithSigner(rsa)},
		},
		{
			name: "OptSignEntitySelector",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			opts: []SignOpt{OptSignEntitySelector(es)},
		},
		{
			name: "OptSignGroup",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			opts: []SignOpt{OptSignWithSigner(ed25519), OptSignGroup(1)},
		},
		{
			name: "OptSignObjects",
			path: filepath.Join("..", "..", "..", "test", "images", "one-group.sif"),
			opts: []SignOpt{OptSignWithSigner(ed25519), OptSignObjects(1)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Signing modifies the file, so work with a temporary file.
			path, err := tempFileFrom(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(path)

			if got, want := Sign(context.Background(), path, tt.opts...), tt.wantErr; !errors.Is(got, want) {
				t.Errorf("got error %v, want %v", got, want)
			}
		})
	}
}
