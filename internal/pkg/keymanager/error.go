// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the LICENSE.md file
// distributed with the sources of this project regarding your rights to use or distribute this
// software.

package keymanager

import (
	"errors"
	"fmt"
	"net/http"

	jsonresp "github.com/sylabs/json-resp"
)

// HTTPError represents an error returned from an HTTP server.
type HTTPError struct {
	code int
	err  error
}

// Code returns the HTTP status code associated with e.
func (e *HTTPError) Code() int { return e.code }

// Unwrap returns the error wrapped by e.
func (e *HTTPError) Unwrap() error { return e.err }

// Error returns a human-readable representation of e.
func (e *HTTPError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%v %v: %v", e.code, http.StatusText(e.code), e.err.Error())
	}
	return fmt.Sprintf("%v %v", e.code, http.StatusText(e.code))
}

// Is compares e against target. If target is a HTTPError with the same code as e, true is returned.
func (e *HTTPError) Is(target error) bool {
	t, ok := target.(*HTTPError)
	return ok && (t.code == e.code)
}

// errorFromResponse returns an HTTPError containing the status code and detailed error message (if
// available) from res.
func errorFromResponse(res *http.Response) error {
	httpErr := HTTPError{code: res.StatusCode}

	var jerr *jsonresp.Error
	if err := jsonresp.ReadError(res.Body); errors.As(err, &jerr) {
		httpErr.err = errors.New(jerr.Message)
	}

	return &httpErr
}
