// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cryptkey

import (
	"fmt"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/pkg/errors"
)

const (
	invalidPemPath = "nothing"
	testPassphrase = "test"
	badPemData     = "bad data"
	goodPemData    = `-----BEGIN RSA PUBLIC KEY-----
MIICCgKCAgEAj29RUJcaXFzKKFhfzZpUTZLf5gc4G+hJbRgOxKiqxlbrTXS2sO73
W38KBs5+ZAj4JUfrxNbUYU9ZVFs4ikHhCIIblMUl5JCGYF00F0nDHIuaRPv5ywI/
Sf6A/+6JA2rDxzvp3C4g2ukPQCrCA+A8hCuM3Qbzv8TR4wGmt0k5SRqfIk2AWAaH
5Dk2bbWzkcLvwRN97/JO5XLrXxiYB1dU7a6tkA+4PChxieJK2y0DxWvPrBsijqj7
j2mogo5FKKqwZ91+2CtDhehDzrshcYdUkGDgjVYH4CNG1Wcw/o0jq3hyIIWteCXT
AmgrGbOfLy+zZq+QkxjxjLFRFm/6L26OMbtb2mjdpU6KbCJJvMhmBW7TwkbKYVIe
gEmb846oRchgG3H/uoR3tPyW6Q5I60S1+S3UQ9xTUNYeXgK9/PTH7w/hsSgqQRUP
HYVU9MBHplHFs+rpLqVjkB90cVaIJ7yVoErJJt/GgHJs0wypopOJ9y1xoK1G/FEv
m/lws02svkAKjIyQiCO3oyCXBa9C4EeATriKt0DvCBh2xM64drbMk5FqvEELKbno
gK3HyFm6tn6tqO0GsLuFYDPPJ8s96OqoTDOXvCNZUQW93ljLOvf8hKQjiueL7nCN
r3Oy11/EgEv3gdQeZ47PKgkevS5vqcT06KZKcIOsnz05ik9WPhZqTW8CAwEAAQ==
-----END RSA PUBLIC KEY-----`
)

func TestNewPlaintextKey(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tests := []struct {
		name          string
		keyInfo       KeyInfo
		expectedError error
	}{
		{
			name:          "unknown format",
			keyInfo:       KeyInfo{Format: Unknown},
			expectedError: ErrUnsupportedKeyURI,
		},
		{
			name:          "passphrase",
			keyInfo:       KeyInfo{Format: Passphrase, Material: testPassphrase},
			expectedError: nil,
		},
		{
			name:          "invalid pem",
			keyInfo:       KeyInfo{Format: PEM, Path: invalidPemPath},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPlaintextKey(tt.keyInfo)
			// We do not always use predefined errors so when dealing with errors, we compare the text associated
			// to the error.
			if (err != nil && tt.expectedError != nil && err.Error() != tt.expectedError.Error()) ||
				((err == nil || tt.expectedError == nil) && err != tt.expectedError) {
				t.Fatalf("test %s returned an unexpected error: %s vs. %s", tt.name, err, tt.expectedError)
			}
		})
	}
}

func TestEncryptKey(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tests := []struct {
		name          string
		keyInfo       KeyInfo
		plaintext     []byte
		expectedError error
	}{
		{
			name:          "unknown format",
			keyInfo:       KeyInfo{Format: Unknown},
			plaintext:     []byte(""),
			expectedError: ErrUnsupportedKeyURI,
		},
		{
			name:          "passphrase",
			keyInfo:       KeyInfo{Format: Passphrase, Material: testPassphrase},
			plaintext:     []byte(""),
			expectedError: nil,
		},
		{
			name:          "invalid pem",
			keyInfo:       KeyInfo{Format: PEM, Path: invalidPemPath},
			plaintext:     []byte(""),
			expectedError: errors.Wrap(fmt.Errorf("open nothing: no such file or directory"), "loading public key for key encryption: loading public key for key encryption"),
		},
		{
			name:          "invalid pem data",
			keyInfo:       KeyInfo{Format: ENV, Material: badPemData},
			plaintext:     []byte(""),
			expectedError: fmt.Errorf("loading public key for key encryption: loading public key for key encryption: could not read bad data: no PEM data"),
		},
		{
			name:          "valid pem data",
			keyInfo:       KeyInfo{Format: ENV, Material: goodPemData},
			plaintext:     []byte(""),
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EncryptKey(tt.keyInfo, tt.plaintext)
			// We do not always use predefined errors so when dealing with errors, we compare the text associated
			// to the error.
			if (err != nil && tt.expectedError != nil && err.Error() != tt.expectedError.Error()) ||
				((err == nil || tt.expectedError == nil) && err != tt.expectedError) {
				t.Fatalf("test %s returned an unexpected error: %s vs. %s", tt.name, err, tt.expectedError)
			}
		})
	}
}

func TestPlaintextKey(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	// TestPlaintestKey reads a key from an image. Creating an image does not
	// fit with unit tests testing so we only test error cases here.
	const (
		noimage = ""
	)

	tests := []struct {
		name          string
		keyInfo       KeyInfo
		expectedError error
	}{
		{
			name:          "unknown format",
			keyInfo:       KeyInfo{Format: Unknown},
			expectedError: ErrUnsupportedKeyURI,
		},
		{
			name:          "passphrase",
			keyInfo:       KeyInfo{Format: Passphrase, Material: testPassphrase},
			expectedError: nil,
		},
		{
			name:          "invalid pem",
			keyInfo:       KeyInfo{Format: PEM, Path: invalidPemPath},
			expectedError: fmt.Errorf("could not load PEM private key: loading public key for key encryption: open nothing: no such file or directory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PlaintextKey(tt.keyInfo, noimage)
			// We do not always use predefined errors so when dealing with errors, we compare the text associated
			// to the error.
			if (err != nil && tt.expectedError != nil && err.Error() != tt.expectedError.Error()) ||
				((err == nil || tt.expectedError == nil) && err != tt.expectedError) {
				t.Fatalf("test %s returned an unexpected error: %s vs. %s", tt.name, err, tt.expectedError)
			}
		})
	}
}
