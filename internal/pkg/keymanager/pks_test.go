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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	jsonresp "github.com/sylabs/json-resp"
)

type MockPKSAdd struct {
	t       *testing.T
	code    int
	message string
	keyText string
}

func (m *MockPKSAdd) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.code/100 != 2 { // non-2xx status code
		if m.message != "" {
			if err := jsonresp.WriteError(w, m.message, m.code); err != nil {
				m.t.Fatalf("failed to write error: %v", err)
			}
		} else {
			w.WriteHeader(m.code)
		}
		return
	}

	if got, want := r.URL.Path, pathPKSAdd; got != want {
		m.t.Errorf("got path %v, want %v", got, want)
	}

	if got, want := r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"; got != want {
		m.t.Errorf("got content type %v, want %v", got, want)
	}

	if err := r.ParseForm(); err != nil {
		m.t.Fatalf("failed to parse form: %v", err)
	}
	if got, want := r.Form.Get("keytext"), m.keyText; got != want {
		m.t.Errorf("got key text %v, want %v", got, want)
	}
}

func TestPKSAdd(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		keyText string
		code    int
		message string
		wantErr error
	}{
		{
			name:    "OK",
			ctx:     context.Background(),
			keyText: "key",
			code:    http.StatusOK,
		},
		{
			name:    "Accepted",
			ctx:     context.Background(),
			keyText: "key",
			code:    http.StatusAccepted,
		},
		{
			name:    "HTTPError",
			ctx:     context.Background(),
			keyText: "key",
			code:    http.StatusBadRequest,
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "HTTPErrorMessage",
			ctx:     context.Background(),
			keyText: "key",
			code:    http.StatusBadRequest,
			message: "blah",
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "ContextCanceled",
			ctx:     canceled,
			keyText: "key",
			code:    http.StatusOK,
			wantErr: context.Canceled,
		},
		{
			name:    "InvalidKeyText",
			ctx:     context.Background(),
			keyText: "",
			wantErr: ErrInvalidKeyText,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := httptest.NewServer(&MockPKSAdd{
				t:       t,
				code:    tt.code,
				message: tt.message,
				keyText: tt.keyText,
			})
			defer s.Close()

			c, err := NewClient(OptBaseURL(s.URL))
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			err = c.PKSAdd(tt.ctx, tt.keyText)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
			}
		})
	}
}

type MockPKSLookup struct {
	t             *testing.T
	code          int
	message       string
	search        string
	op            string
	options       string
	fingerprint   bool
	exact         bool
	pageSize      string
	pageToken     string
	nextPageToken string
	response      string
}

func (m *MockPKSLookup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.code/100 != 2 { // non-2xx status code
		if m.message != "" {
			if err := jsonresp.WriteError(w, m.message, m.code); err != nil {
				m.t.Fatalf("failed to write error: %v", err)
			}
		} else {
			w.WriteHeader(m.code)
		}
		return
	}

	if got, want := r.URL.Path, pathPKSLookup; got != want {
		m.t.Errorf("got path %v, want %v", got, want)
	}

	if got, want := r.ContentLength, int64(0); got != want {
		m.t.Errorf("got content length %v, want %v", got, want)
	}

	if err := r.ParseForm(); err != nil {
		m.t.Fatalf("failed to parse form: %v", err)
	}
	if got, want := r.Form.Get("search"), m.search; got != want {
		m.t.Errorf("got search %v, want %v", got, want)
	}
	if got, want := r.Form.Get("op"), m.op; got != want {
		m.t.Errorf("got op %v, want %v", got, want)
	}

	// options is optional.
	options, ok := r.Form["options"]
	if got, want := ok, m.options != ""; got != want {
		m.t.Errorf("options presence %v, want %v", got, want)
	} else if ok {
		if len(options) != 1 {
			m.t.Errorf("got multiple options values")
		} else if got, want := options[0], m.options; got != want {
			m.t.Errorf("got options %v, want %v", got, want)
		}
	}

	// fingerprint is optional.
	fp, ok := r.Form["fingerprint"]
	if got, want := ok, m.fingerprint; got != want {
		m.t.Errorf("fingerprint presence %v, want %v", got, want)
	} else if ok {
		if len(fp) != 1 {
			m.t.Errorf("got multiple fingerprint values")
		} else if got, want := fp[0], "on"; got != want {
			m.t.Errorf("got fingerprint %v, want %v", got, want)
		}
	}

	// exact is optional.
	exact, ok := r.Form["exact"]
	if got, want := ok, m.exact; got != want {
		m.t.Errorf("exact presence %v, want %v", got, want)
	} else if ok {
		if len(exact) != 1 {
			m.t.Errorf("got multiple exact values")
		} else if got, want := exact[0], "on"; got != want {
			m.t.Errorf("got exact %v, want %v", got, want)
		}
	}

	// x-pagesize is optional.
	pageSize, ok := r.Form["x-pagesize"]
	if got, want := ok, m.pageSize != ""; got != want {
		m.t.Errorf("page size presence %v, want %v", got, want)
	} else if ok {
		if len(pageSize) != 1 {
			m.t.Error("got multiple page size values")
		} else if got, want := pageSize[0], m.pageSize; got != want {
			m.t.Errorf("got page size %v, want %v", got, want)
		}
	}

	// x-pagetoken is optional.
	pageToken, ok := r.Form["x-pagetoken"]
	if got, want := ok, m.pageToken != ""; got != want {
		m.t.Errorf("page token presence %v, want %v", got, want)
	} else if ok {
		if len(pageToken) != 1 {
			m.t.Error("got multiple page token values")
		} else if got, want := pageToken[0], m.pageToken; got != want {
			m.t.Errorf("got page token %v, want %v", got, want)
		}
	}

	w.Header().Set("X-HKP-Next-Page-Token", m.nextPageToken)
	if _, err := io.Copy(w, strings.NewReader(m.response)); err != nil {
		m.t.Fatalf("failed to copy: %v", err)
	}
}

func TestPKSLookup(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name          string
		ctx           context.Context
		code          int
		message       string
		search        string
		op            string
		options       []string
		fingerprint   bool
		exact         bool
		pageToken     string
		pageSize      int
		nextPageToken string
		wantErr       error
	}{
		{
			name:   "Get",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: "search",
			op:     OperationGet,
		},
		{
			name:          "GetNPT",
			ctx:           context.Background(),
			code:          http.StatusOK,
			search:        "search",
			op:            OperationGet,
			nextPageToken: "bar",
		},
		{
			name:     "GetSize",
			ctx:      context.Background(),
			code:     http.StatusOK,
			search:   "search",
			op:       OperationGet,
			pageSize: 42,
		},
		{
			name:          "GetSizeNPT",
			ctx:           context.Background(),
			code:          http.StatusOK,
			search:        "search",
			op:            OperationGet,
			pageSize:      42,
			nextPageToken: "bar",
		},
		{
			name:      "GetPT",
			ctx:       context.Background(),
			code:      http.StatusOK,
			search:    "search",
			op:        OperationGet,
			pageToken: "foo",
		},
		{
			name:          "GetPTNPT",
			ctx:           context.Background(),
			code:          http.StatusOK,
			search:        "search",
			op:            OperationGet,
			pageToken:     "foo",
			nextPageToken: "bar",
		},
		{
			name:      "GetPTSize",
			ctx:       context.Background(),
			code:      http.StatusOK,
			search:    "search",
			op:        OperationGet,
			pageToken: "foo",
			pageSize:  42,
		},
		{
			name:          "GetPTSizeNPT",
			ctx:           context.Background(),
			code:          http.StatusOK,
			search:        "search",
			op:            OperationGet,
			pageToken:     "foo",
			pageSize:      42,
			nextPageToken: "bar",
		},
		{
			name:    "GetMachineReadable",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			op:      OperationGet,
			options: []string{OptionMachineReadable},
		},
		{
			name:    "GetMachineReadableBlah",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			op:      OperationGet,
			options: []string{OptionMachineReadable, "blah"},
		},
		{
			name:   "GetExact",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: "search",
			op:     OperationGet,
			exact:  true,
		},
		{
			name:   "Index",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: "search",
			op:     OperationIndex,
		},
		{
			name:    "IndexMachineReadable",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			op:      OperationIndex,
			options: []string{OptionMachineReadable},
		},
		{
			name:    "IndexMachineReadableBlah",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			op:      OperationIndex,
			options: []string{OptionMachineReadable, "blah"},
		},
		{
			name:        "IndexFingerprint",
			ctx:         context.Background(),
			code:        http.StatusOK,
			search:      "search",
			op:          OperationIndex,
			fingerprint: true,
		},
		{
			name:   "IndexExact",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: "search",
			op:     OperationIndex,
			exact:  true,
		},
		{
			name:   "VIndex",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: "search",
			op:     OperationVIndex,
		},
		{
			name:    "VIndexMachineReadable",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			op:      OperationVIndex,
			options: []string{OptionMachineReadable},
		},
		{
			name:    "VIndexMachineReadableBlah",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			op:      OperationVIndex,
			options: []string{OptionMachineReadable, "blah"},
		},
		{
			name:        "VIndexFingerprint",
			ctx:         context.Background(),
			code:        http.StatusOK,
			search:      "search",
			op:          OperationVIndex,
			fingerprint: true,
		},
		{
			name:   "VIndexExact",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: "search",
			op:     OperationVIndex,
			exact:  true,
		},
		{
			name:   "NonAuthoritativeInfo",
			ctx:    context.Background(),
			code:   http.StatusNonAuthoritativeInfo,
			search: "search",
			op:     OperationGet,
		},
		{
			name:    "HTTPError",
			ctx:     context.Background(),
			code:    http.StatusBadRequest,
			search:  "search",
			op:      OperationGet,
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "HTTPErrorMessage",
			ctx:     context.Background(),
			code:    http.StatusBadRequest,
			message: "blah",
			search:  "search",
			op:      OperationGet,
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "ContextCanceled",
			ctx:     canceled,
			code:    http.StatusOK,
			search:  "search",
			op:      OperationGet,
			wantErr: context.Canceled,
		},
		{
			name:    "InvalidSearch",
			ctx:     context.Background(),
			code:    http.StatusOK,
			op:      OperationGet,
			wantErr: ErrInvalidSearch,
		},
		{
			name:    "InvalidOperation",
			ctx:     context.Background(),
			code:    http.StatusOK,
			search:  "search",
			options: []string{},
			wantErr: ErrInvalidOperation,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := MockPKSLookup{
				t:             t,
				response:      "Not valid, but it'll do for testing",
				code:          tt.code,
				message:       tt.message,
				search:        tt.search,
				op:            tt.op,
				options:       strings.Join(tt.options, ","),
				fingerprint:   tt.fingerprint,
				exact:         tt.exact,
				pageToken:     tt.pageToken,
				nextPageToken: tt.nextPageToken,
			}
			if tt.pageSize != 0 {
				m.pageSize = strconv.Itoa(tt.pageSize)
			}

			s := httptest.NewServer(&m)
			defer s.Close()

			c, err := NewClient(OptBaseURL(s.URL))
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			pd := PageDetails{
				Token: tt.pageToken,
				Size:  tt.pageSize,
			}
			r, err := c.PKSLookup(tt.ctx, &pd, tt.search, tt.op, tt.fingerprint, tt.exact, tt.options)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
			}

			if err == nil {
				if got, want := pd.Token, tt.nextPageToken; got != want {
					t.Errorf("got page token %v, want %v", got, want)
				}
				if got, want := r, m.response; got != want {
					t.Errorf("got response %v, want %v", got, want)
				}
			}
		})
	}
}

func TestGetKey(t *testing.T) {
	search := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13,
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		code    int
		message string
		search  []byte
		wantErr error
	}{
		{
			name:   "ShortKeyID",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: search[len(search)-4:],
		},
		{
			name:   "KeyID",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: search[len(search)-8:],
		},
		{
			name:   "V3Fingerprint",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: search[len(search)-16:],
		},
		{
			name:   "V4Fingerprint",
			ctx:    context.Background(),
			code:   http.StatusOK,
			search: search,
		},
		{
			name:   "NonAuthoritativeInfo",
			ctx:    context.Background(),
			code:   http.StatusNonAuthoritativeInfo,
			search: search,
		},
		{
			name:    "HTTPError",
			ctx:     context.Background(),
			code:    http.StatusBadRequest,
			search:  search,
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "HTTPErrorMessage",
			ctx:     context.Background(),
			code:    http.StatusBadRequest,
			message: "blah",
			search:  search,
			wantErr: &HTTPError{code: http.StatusBadRequest},
		},
		{
			name:    "ContextCanceled",
			ctx:     canceled,
			code:    http.StatusOK,
			search:  search,
			wantErr: context.Canceled,
		},
		{
			name:    "InvalidSearch",
			ctx:     context.Background(),
			search:  search[:1],
			wantErr: ErrInvalidSearch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := MockPKSLookup{
				t:        t,
				code:     tt.code,
				message:  tt.message,
				search:   fmt.Sprintf("%#x", tt.search),
				op:       OperationGet,
				exact:    true,
				response: "Not valid, but it'll do for testing",
			}

			s := httptest.NewServer(&m)
			defer s.Close()

			c, err := NewClient(OptBaseURL(s.URL))
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			kt, err := c.GetKey(tt.ctx, tt.search)

			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
			}

			if err == nil {
				if got, want := kt, m.response; got != want {
					t.Errorf("got keyText %v, want %v", got, want)
				}
			}
		})
	}
}
