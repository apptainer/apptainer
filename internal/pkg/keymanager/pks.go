// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package keymanager

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	pathPKSAdd    = "/pks/add"
	pathPKSLookup = "/pks/lookup"
)

// ErrInvalidKeyText is returned when the key text is invalid.
var ErrInvalidKeyText = errors.New("invalid key text")

// ErrInvalidSearch is returned when the search value is invalid.
var ErrInvalidSearch = errors.New("invalid search")

// ErrInvalidOperation is returned when the operation is invalid.
var ErrInvalidOperation = errors.New("invalid operation")

// PKSAdd submits an ASCII armored keyring to the Key Service, as specified in section 4 of the
// OpenPGP HTTP Keyserver Protocol (HKP) specification. The context controls the lifetime of the
// request.
//
// If an non-200 HTTP status code is received, an error wrapping an HTTPError is returned.
func (c *Client) PKSAdd(ctx context.Context, keyText string) error {
	if keyText == "" {
		return fmt.Errorf("%w", ErrInvalidKeyText)
	}

	ref := &url.URL{Path: pathPKSAdd}

	v := url.Values{}
	v.Set("keytext", keyText)

	req, err := c.NewRequest(ctx, http.MethodPost, ref, strings.NewReader(v.Encode()))
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 { // non-2xx status code
		return fmt.Errorf("%w", errorFromResponse(res))
	}
	return nil
}

// PageDetails includes pagination details.
type PageDetails struct {
	// Maximum number of results per page (server may ignore or return fewer).
	Size int
	// Token for next page (advanced with each request, empty for last page).
	Token string
}

const (
	// OperationGet is a PKSLookup operation value to perform a "get" operation.
	OperationGet = "get"
	// OperationIndex is a PKSLookup operation value to perform a "index" operation.
	OperationIndex = "index"
	// OperationVIndex is a PKSLookup operation value to perform a "vindex" operation.
	OperationVIndex = "vindex"
)

// OptionMachineReadable is a PKSLookup options value to return machine readable output.
const OptionMachineReadable = "mr"

// PKSLookup requests data from the Key Service, as specified in section 3 of the OpenPGP HTTP
// Keyserver Protocol (HKP) specification. The context controls the lifetime of the request.
//
// If an non-200 HTTP status code is received, an error wrapping an HTTPError is returned.
func (c *Client) PKSLookup(ctx context.Context, pd *PageDetails, search, operation string, fingerprint, exact bool, options []string) (response string, err error) {
	if search == "" {
		return "", fmt.Errorf("%w", ErrInvalidSearch)
	}
	if operation == "" {
		return "", fmt.Errorf("%w", ErrInvalidOperation)
	}

	v := url.Values{}
	v.Set("search", search)
	v.Set("op", operation)
	if 0 < len(options) {
		v.Set("options", strings.Join(options, ","))
	}
	if fingerprint {
		v.Set("fingerprint", "on")
	}
	if exact {
		v.Set("exact", "on")
	}
	if pd != nil {
		if pd.Size != 0 {
			v.Set("x-pagesize", strconv.Itoa(pd.Size))
		}
		if pd.Token != "" {
			v.Set("x-pagetoken", pd.Token)
		}
	}

	ref := &url.URL{Path: pathPKSLookup, RawQuery: v.Encode()}

	req, err := c.NewRequest(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	res, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 { // non-2xx status code
		return "", fmt.Errorf("%w", errorFromResponse(res))
	}

	if pd != nil {
		pd.Token = res.Header.Get("X-HKP-Next-Page-Token")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return string(body), nil
}

// GetKey retrieves an ASCII armored keyring matching search from the Key Service. A 32-bit key ID,
// 64-bit key ID, 128-bit version 3 fingerprint, or 160-bit version 4 fingerprint can be specified
// in search. The context controls the lifetime of the request.
//
// If an non-200 HTTP status code is received, an error wrapping an HTTPError is returned.
func (c *Client) GetKey(ctx context.Context, search []byte) (keyText string, err error) {
	switch len(search) {
	case 4, 8, 16, 20:
		return c.PKSLookup(ctx, nil, fmt.Sprintf("%#x", search), OperationGet, false, true, nil)
	default:
		return "", fmt.Errorf("%w", ErrInvalidSearch)
	}
}
