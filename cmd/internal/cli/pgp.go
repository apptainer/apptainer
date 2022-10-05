// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package cli

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/util/interactive"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/apptainer/sif/v2/pkg/sif"
)

var (
	errEmptyKeyring    = errors.New("keyring is empty")
	errIndexOutOfRange = errors.New("index out of range")
)

// printEntityAtIndex prints entity e, associated with index i, to w.
func printEntityAtIndex(w io.Writer, i int, e *openpgp.Entity) {
	for _, v := range e.Identities {
		fmt.Fprintf(w, "%d) U: %s (%s) <%s>\n", i, v.UserId.Name, v.UserId.Comment, v.UserId.Email)
	}
	fmt.Fprintf(w, "   C: %s\n", e.PrimaryKey.CreationTime)
	fmt.Fprintf(w, "   F: %0X\n", e.PrimaryKey.Fingerprint)
	bits, _ := e.PrimaryKey.BitLength()
	fmt.Fprintf(w, "   L: %d\n", bits)
	fmt.Fprint(os.Stdout, "   --------\n")
}

// selectEntityInteractive returns an EntitySelector that selects an entity from el, prompting the
// user for a selection if there is more than one entity in el.
func selectEntityInteractive() sypgp.EntitySelector {
	return func(el openpgp.EntityList) (*openpgp.Entity, error) {
		switch len(el) {
		case 0:
			return nil, errEmptyKeyring
		case 1:
			return el[0], nil
		default:
			for i, e := range el {
				printEntityAtIndex(os.Stdout, i, e)
			}

			n, err := interactive.AskNumberInRange(0, len(el)-1, "Enter # of private key to use : ")
			if err != nil {
				return nil, err
			}
			return el[n], nil
		}
	}
}

// selectEntityAtIndex returns an EntitySelector that selects the entity at index i.
func selectEntityAtIndex(i int) sypgp.EntitySelector {
	return func(el openpgp.EntityList) (*openpgp.Entity, error) {
		if i >= len(el) {
			return nil, errIndexOutOfRange
		}
		return el[i], nil
	}
}

// decryptSelectedEntityInteractive wraps f, attempting to decrypt the private key in the selected
// entity with a passpharse provided interactively by the user.
func decryptSelectedEntityInteractive(f sypgp.EntitySelector) sypgp.EntitySelector {
	return func(el openpgp.EntityList) (*openpgp.Entity, error) {
		e, err := f(el)
		if err != nil {
			return nil, err
		}

		if e.PrivateKey.Encrypted {
			if err := decryptPrivateKeyInteractive(e); err != nil {
				return nil, err
			}
		}

		return e, nil
	}
}

// decryptPrivateKeyInteractive decrypts the private key in e, prompting the user for a passphrase.
func decryptPrivateKeyInteractive(e *openpgp.Entity) error {
	passphrase, err := interactive.AskQuestionNoEcho("Enter key passphrase : ")
	if err != nil {
		return err
	}

	return e.PrivateKey.Decrypt([]byte(passphrase))
}

// primaryIdentity returns the Identity marked as primary, or the first identity if none are so
// marked.
func primaryIdentity(e *openpgp.Entity) *openpgp.Identity {
	var first *openpgp.Identity
	for _, id := range e.Identities {
		if first == nil {
			first = id
		}
		if id.SelfSignature.IsPrimaryId != nil && *id.SelfSignature.IsPrimaryId {
			return id
		}
	}
	return first
}

// isLocal returns true if signing entity e is found in the local keyring, and false otherwise.
func isLocal(e *openpgp.Entity) bool {
	kr, err := sypgp.PublicKeyRing()
	if err != nil {
		return false
	}

	keys := kr.KeysByIdUsage(e.PrimaryKey.KeyId, packet.KeyFlagSign)
	return len(keys) > 0
}

// isGlobal returns true if signing entity e is found in the global keyring, and false otherwise.
func isGlobal(e *openpgp.Entity) bool {
	keyring := sypgp.NewHandle(buildcfg.APPTAINER_CONFDIR, sypgp.GlobalHandleOpt())
	kr, err := keyring.LoadPubKeyring()
	if err != nil {
		return false
	}

	keys := kr.KeysByIdUsage(e.PrimaryKey.KeyId, packet.KeyFlagSign)
	return len(keys) > 0
}

type key struct {
	Signer keyEntity
}

// keyEntity holds all the key info, used for json output.
type keyEntity struct {
	Partition   string
	Name        string
	Fingerprint string
	KeyLocal    bool
	KeyCheck    bool
	DataCheck   bool
}

// keyList is a list of one or more keys.
type keyList struct {
	Signatures int
	SignerKeys []*key
}

// getJSONCallback returns an apptainer.VerifyCallback that appends to kl.
func getJSONCallback(kl *keyList) apptainer.VerifyCallback {
	return func(f *sif.FileImage, r integrity.VerifyResult) bool {
		name, fp := "unknown", ""
		var keyLocal, keyCheck bool

		// Increment signature count.
		kl.Signatures++

		// If entity is determined, note a few values.
		if e := r.Entity().(*openpgp.Entity); e != nil {
			if id := primaryIdentity(e); id != nil {
				name = id.Name
			}
			fp = hex.EncodeToString(e.PrimaryKey.Fingerprint[:])
			keyLocal = isLocal(e)
			keyCheck = true
		}

		// For each verified object, append an entry to the list.
		for _, od := range r.Verified() {
			ke := keyEntity{
				Partition:   od.DataType().String(),
				Name:        name,
				Fingerprint: fp,
				KeyLocal:    keyLocal,
				KeyCheck:    keyCheck,
				DataCheck:   true,
			}
			kl.SignerKeys = append(kl.SignerKeys, &key{ke})
		}

		var integrityError *integrity.ObjectIntegrityError
		if errors.As(r.Error(), &integrityError) {
			od, err := f.GetDescriptor(sif.WithID(integrityError.ID))
			if err != nil {
				sylog.Errorf("failed to get descriptor: %v", err)
				return false
			}

			ke := keyEntity{
				Partition:   od.DataType().String(),
				Name:        name,
				Fingerprint: fp,
				KeyLocal:    keyLocal,
				KeyCheck:    keyCheck,
				DataCheck:   false,
			}
			kl.SignerKeys = append(kl.SignerKeys, &key{ke})
		}

		return false
	}
}

// outputJSON outputs a JSON representation of kl to w.
func outputJSON(w io.Writer, kl keyList) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(kl)
}
