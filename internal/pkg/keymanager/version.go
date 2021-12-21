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
	"fmt"
	"net/http"
	"net/url"

	jsonresp "github.com/sylabs/json-resp"
)

const pathVersion = "version"

// GetVersion gets version information from the Key Service. The context controls the lifetime of
// the request.
//
// If an non-200 HTTP status code is received, an error wrapping an HTTPError is returned.
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	ref := &url.URL{Path: pathVersion}

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

	vi := struct {
		Version string `json:"version"`
	}{}
	if err := jsonresp.ReadResponse(res.Body, &vi); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return vi.Version, nil
}
