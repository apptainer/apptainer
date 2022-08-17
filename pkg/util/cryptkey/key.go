// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cryptkey

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/apptainer/sif/v2/pkg/sif"
)

var (
	// ErrEncryptedKeyNotFound indicates the encrypted key is not found.
	ErrEncryptedKeyNotFound = errors.New("encrypted key not found")
	// ErrUnsupportedKeyURI indicates the key URI is not supported.
	ErrUnsupportedKeyURI = errors.New("unsupported key URI")
	// ErrNoEncryptedKeyData indicates there is no encrypted key data.
	ErrNoEncryptedKeyData = errors.New("no encrypted key data")
	// ErrNoPEMData indicates there is no PEM data.
	ErrNoPEMData = errors.New("no PEM data")
)

const (
	// Unknown indicates the key material format is not known.
	Unknown = iota
	// Passphrase indicates the key material is formatted as a passphrase.
	Passphrase
	// PEM indicates the key material is formatted as a PEM file.
	PEM
)

// KeyInfo contains information for passing around
// or extracting a passphrase for an encrypted container
type KeyInfo struct {
	Format   int
	Material string
	Path     string
}

func getRandomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func NewPlaintextKey(k KeyInfo) ([]byte, error) {
	switch k.Format {
	case PEM:
		// in this case we will generate a random secret and
		// encrypt it using the PEM key.use the PEM key to
		// encrypt a secret
		return getRandomBytes(64)

	case Passphrase:
		// return the original value unmodified
		return []byte(k.Material), nil

	default:
		return nil, ErrUnsupportedKeyURI
	}
}

func EncryptKey(k KeyInfo, plaintext []byte) ([]byte, error) {
	switch k.Format {
	case PEM:
		pubKey, err := LoadPEMPublicKey(k.Path)
		if err != nil {
			return nil, fmt.Errorf("loading public key for key encryption: %v", err)
		}

		ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, plaintext, nil)
		if err != nil {
			return nil, fmt.Errorf("encrypting key: %v", err)
		}

		var buf bytes.Buffer

		if err := savePEMMessage(&buf, ciphertext); err != nil {
			return nil, fmt.Errorf("serializing encrypted key: %v", err)
		}

		return buf.Bytes(), nil

	case Passphrase:
		return nil, nil

	default:
		return nil, ErrUnsupportedKeyURI
	}
}

func PlaintextKey(k KeyInfo, image string) ([]byte, error) {
	switch k.Format {
	case PEM:
		privateKey, err := LoadPEMPrivateKey(k.Path)
		if err != nil {
			return nil, fmt.Errorf("could not load PEM private key: %v", err)
		}

		pemKey, err := getEncryptionKeyFromImage(image)
		if err != nil {
			return nil, fmt.Errorf("could not get encryption information from SIF: %v", err)
		}

		pemBuf := bytes.NewReader(pemKey)

		encKey, err := loadPEMMessage(pemBuf)
		if err != nil {
			return nil, fmt.Errorf("could not unpack LUKS PEM from SIF: %v", err)
		}

		plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encKey, nil)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt LUKS key: %v", err)
		}

		return plaintext, nil

	case Passphrase:
		return []byte(k.Material), nil

	default:
		return nil, ErrUnsupportedKeyURI
	}
}

func LoadPEMPrivateKey(fn string) (*rsa.PrivateKey, error) {
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("could not read %s: %v", fn, ErrNoPEMData)
	}

	if strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
		return nil, fmt.Errorf("passphrase protected pem files not supported")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func LoadPEMPublicKey(fn string) (*rsa.PublicKey, error) {
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("could not read %s: %v", fn, ErrNoPEMData)
	}

	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func loadPEMMessage(r io.Reader) ([]byte, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("could not load decode PEM key %s: %v", r, ErrNoPEMData)
	}

	var buf []byte
	if _, err := asn1.Unmarshal(block.Bytes, &buf); err != nil {
		return nil, fmt.Errorf("could not unmarshal key asn1 data: %v", err)
	}

	return buf, nil
}

func savePEMMessage(w io.Writer, msg []byte) error {
	asn1Bytes, err := asn1.Marshal(msg)
	if err != nil {
		return err
	}

	b := &pem.Block{
		Type:  "MESSAGE",
		Bytes: asn1Bytes,
	}

	return pem.Encode(w, b)
}

func getEncryptionKeyFromImage(fn string) ([]byte, error) {
	img, err := sif.LoadContainerFromPath(fn, sif.OptLoadWithFlag(os.O_RDONLY))
	if err != nil {
		return nil, fmt.Errorf("could not load container: %w", err)
	}
	defer img.UnloadContainer()

	primDescr, err := img.GetDescriptor(sif.WithPartitionType(sif.PartPrimSys))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve primary system partition from '%s': %w", fn, err)
	}

	descr, err := img.GetDescriptors(
		sif.WithLinkedID(primDescr.ID()),
		sif.WithDataType(sif.DataCryptoMessage),
	)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve linked descriptors for primary system partition from %s: %w", fn, err)
	}

	for _, d := range descr {
		format, message, err := d.CryptoMessageMetadata()
		if err != nil {
			return nil, fmt.Errorf("could not get crypto message metadata: %w", err)
		}

		// currently only support one type of message
		if format != sif.FormatPEM || message != sif.MessageRSAOAEP {
			continue
		}

		// TODO(ian): For now, assume the first linked message is what we
		// are looking for. We should consider what we want to do in the
		// case of multiple linked messages
		key, err := d.GetData()
		if err != nil {
			return nil, fmt.Errorf("could not retrieve LUKS key data from %s: %w", fn, err)
		}

		return key, nil
	}

	return nil, fmt.Errorf("could not read LUKS key from %s: %v", fn, ErrEncryptedKeyNotFound)
}
