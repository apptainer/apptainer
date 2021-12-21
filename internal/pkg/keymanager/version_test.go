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
	"net/http"
	"net/http/httptest"
	"testing"

	jsonresp "github.com/sylabs/json-resp"
)

type MockVersion struct {
	t        *testing.T
	code     int
	message  string
	wantPath string
	version  string
}

func (m *MockVersion) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.code/100 != 2 { // non-2xx status code
		if err := jsonresp.WriteError(w, m.message, m.code); err != nil {
			m.t.Fatalf("failed to write error: %v", err)
		}
		return
	}

	if got, want := r.URL.Path, m.wantPath; got != want {
		m.t.Errorf("got path %v, want %v", got, want)
	}

	if got, want := r.ContentLength, int64(0); got != want {
		m.t.Errorf("got content length %v, want %v", got, want)
	}

	vi := struct {
		Version string `json:"version"`
	}{
		Version: m.version,
	}
	if err := jsonresp.WriteResponse(w, vi, m.code); err != nil {
		m.t.Fatalf("failed to write response: %v", err)
	}
}

func TestGetVersion(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name     string
		path     string
		ctx      context.Context
		code     int
		message  string
		wantPath string
		version  string
		wantErr  error
	}{
		{
			name:     "OK",
			ctx:      context.Background(),
			code:     http.StatusOK,
			wantPath: "/version",
			version:  "1.2.3",
		},
		{
			name:     "OKWithPath",
			path:     "/path",
			ctx:      context.Background(),
			code:     http.StatusOK,
			wantPath: "/path/version",
			version:  "1.2.3",
		},
		{
			name:     "NonAuthoritativeInfo",
			ctx:      context.Background(),
			code:     http.StatusNonAuthoritativeInfo,
			wantPath: "/version",
			version:  "1.2.3",
		},
		{
			name:    "HTTPError",
			ctx:     context.Background(),
			code:    http.StatusBadRequest,
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "HTTPErrorMessage",
			ctx:     context.Background(),
			code:    http.StatusBadRequest,
			message: "blah",
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "ContextCanceled",
			ctx:     canceled,
			code:    http.StatusOK,
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := httptest.NewServer(&MockVersion{
				t:        t,
				code:     tt.code,
				message:  tt.message,
				wantPath: tt.wantPath,
				version:  tt.version,
			})
			defer s.Close()

			c, err := NewClient(OptBaseURL(s.URL + tt.path))
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			v, err := c.GetVersion(tt.ctx)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
			}

			if err == nil {
				if got, want := v, tt.version; got != want {
					t.Errorf("got version %v, want %v", got, want)
				}
			}
		})
	}
}
