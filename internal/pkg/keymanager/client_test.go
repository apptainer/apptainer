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
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		wantURL *url.URL
	}{
		{"BadURL", ":", true, nil},
		{"BadScheme", "bad:", true, nil},
		{"HTTPBaseURL", "http://p80.pool.sks-keyservers.net", false, &url.URL{
			Scheme: "http",
			Host:   "p80.pool.sks-keyservers.net",
			Path:   "/",
		}},
		{"HTTPSBaseURL", "https://hkps.pool.sks-keyservers.net", false, &url.URL{
			Scheme: "https",
			Host:   "hkps.pool.sks-keyservers.net",
			Path:   "/",
		}},
		{"HKPBaseURL", "hkp://pool.sks-keyservers.net", false, &url.URL{
			Scheme: "http",
			Host:   "pool.sks-keyservers.net:11371",
			Path:   "/",
		}},
		{"HKPSBaseURL", "hkps://hkps.pool.sks-keyservers.net", false, &url.URL{
			Scheme: "https",
			Host:   "hkps.pool.sks-keyservers.net",
			Path:   "/",
		}},
		{"BaseURLSlash", "hkps://hkps.pool.sks-keyservers.net/", false, &url.URL{
			Scheme: "https",
			Host:   "hkps.pool.sks-keyservers.net",
			Path:   "/",
		}},
		{"BaseURLPath", "hkps://hkps.pool.sks-keyservers.net/path", false, &url.URL{
			Scheme: "https",
			Host:   "hkps.pool.sks-keyservers.net",
			Path:   "/path/",
		}},
		{"BaseURLPathSlash", "hkps://hkps.pool.sks-keyservers.net/path/", false, &url.URL{
			Scheme: "https",
			Host:   "hkps.pool.sks-keyservers.net",
			Path:   "/path/",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := normalizeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}

			if err == nil {
				if got, want := u, tt.wantURL; !reflect.DeepEqual(got, want) {
					t.Errorf("got url %v, want %v", got, want)
				}
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	httpClient := &http.Client{}

	tests := []struct {
		name           string
		opts           []Option
		wantErr        error
		wantURL        string
		bearerToken    string
		wantUserAgent  string
		wantHTTPClient *http.Client
	}{
		{"UnsupportedProtocolScheme", []Option{
			OptBaseURL("bad:"),
		}, errUnsupportedProtocolScheme, "", "", "", nil},
		{"TLSRequiredHTTP", []Option{
			OptBaseURL("http://p80.pool.sks-keyservers.net"),
			OptBearerToken("blah"),
		}, ErrTLSRequired, "", "", "", nil},
		{"TLSRequiredHKP", []Option{
			OptBaseURL("hkp://pool.sks-keyservers.net"),
			OptBearerToken("blah"),
		}, ErrTLSRequired, "", "", "", nil},
		{"Defaults", nil, nil, defaultBaseURL, "", "", http.DefaultClient},
		{"BaseURL", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net"),
		}, nil, "https://hkps.pool.sks-keyservers.net/", "", "", http.DefaultClient},
		{"BaseURLSlash", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net/"),
		}, nil, "https://hkps.pool.sks-keyservers.net/", "", "", http.DefaultClient},
		{"BaseURLWithPath", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net/path"),
		}, nil, "https://hkps.pool.sks-keyservers.net/path/", "", "", http.DefaultClient},
		{"BaseURLWithPathSlash", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net/path/"),
		}, nil, "https://hkps.pool.sks-keyservers.net/path/", "", "", http.DefaultClient},
		{"BearerToken", []Option{
			OptBearerToken("blah"),
		}, nil, defaultBaseURL, "blah", "", http.DefaultClient},
		{"UserAgent", []Option{
			OptUserAgent("Secret Agent Man"),
		}, nil, defaultBaseURL, "", "Secret Agent Man", http.DefaultClient},
		{"HTTPClient", []Option{
			OptHTTPClient(httpClient),
		}, nil, defaultBaseURL, "", "", httpClient},
		{"LocalhostBearerTokenHTTP", []Option{
			OptBaseURL("http://localhost"),
			OptBearerToken("blah"),
		}, nil, "http://localhost/", "blah", "", http.DefaultClient},
		{"LocalhostBearerTokenHTTP8080", []Option{
			OptBaseURL("http://localhost:8080"),
			OptBearerToken("blah"),
		}, nil, "http://localhost:8080/", "blah", "", http.DefaultClient},
		{"LocalhostBearerTokenHKP", []Option{
			OptBaseURL("hkp://localhost"),
			OptBearerToken("blah"),
		}, nil, "http://localhost:11371/", "blah", "", http.DefaultClient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.opts...)
			if got, want := err, tt.wantErr; !errors.Is(got, want) {
				t.Fatalf("got error %v, want %v", got, want)
			}

			if err == nil {
				if got, want := c.baseURL.String(), tt.wantURL; got != want {
					t.Errorf("got host %v, want %v", got, want)
				}

				if got, want := c.bearerToken, tt.bearerToken; got != want {
					t.Errorf("got auth token %v, want %v", got, want)
				}

				if got, want := c.userAgent, tt.wantUserAgent; got != want {
					t.Errorf("got user agent %v, want %v", got, want)
				}

				if got, want := c.httpClient, tt.wantHTTPClient; got != want {
					t.Errorf("got HTTP client %v, want %v", got, want)
				}
			}
		})
	}
}

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name           string
		opts           []Option
		method         string
		path           string
		rawQuery       string
		body           string
		wantErr        bool
		wantURL        string
		wantAuthBearer string
		wantUserAgent  string
	}{
		{"BadMethod", nil, "b@d	", "", "", "", true, "", "", ""},
		{"Get", nil, http.MethodGet, "/path", "", "", false, "https://keys.openpgp.org/path", "", ""},
		{"Post", nil, http.MethodPost, "/path", "", "", false, "https://keys.openpgp.org/path", "", ""},
		{"PostRawQuery", nil, http.MethodPost, "/path", "a=b", "", false, "https://keys.openpgp.org/path?a=b", "", ""},
		{"PostBody", nil, http.MethodPost, "/path", "", "body", false, "https://keys.openpgp.org/path", "", ""},
		{"BaseURLAbsolute", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net"),
		}, http.MethodGet, "/path", "", "", false, "https://hkps.pool.sks-keyservers.net/path", "", ""},
		{"BaseURLRelative", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net"),
		}, http.MethodGet, "path", "", "", false, "https://hkps.pool.sks-keyservers.net/path", "", ""},
		{"BaseURLPathAbsolute", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net/a/b"),
		}, http.MethodGet, "/path", "", "", false, "https://hkps.pool.sks-keyservers.net/path", "", ""},
		{"BaseURLPathRelative", []Option{
			OptBaseURL("hkps://hkps.pool.sks-keyservers.net/a/b"),
		}, http.MethodGet, "path", "", "", false, "https://hkps.pool.sks-keyservers.net/a/b/path", "", ""},
		{"BearerToken", []Option{
			OptBearerToken("blah"),
		}, http.MethodGet, "/path", "", "", false, "https://keys.openpgp.org/path", "BEARER blah", ""},
		{"UserAgent", []Option{
			OptUserAgent("Secret Agent Man"),
		}, http.MethodGet, "/path", "", "", false, "https://keys.openpgp.org/path", "", "Secret Agent Man"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.opts...)
			if err != nil {
				t.Fatal(err)
			}

			ref := &url.URL{Path: tt.path, RawQuery: tt.rawQuery}

			r, err := c.NewRequest(context.Background(), tt.method, ref, strings.NewReader(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				if got, want := r.Method, tt.method; got != want {
					t.Errorf("got method %v, want %v", got, want)
				}

				if got, want := r.URL.String(), tt.wantURL; got != want {
					t.Errorf("got URL %v, want %v", got, want)
				}

				b, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read body: %v", err)
				}
				if got, want := string(b), tt.body; got != want {
					t.Errorf("got body %v, want %v", got, want)
				}

				authBearer, ok := r.Header["Authorization"]
				if got, want := ok, (tt.wantAuthBearer != ""); got != want {
					t.Fatalf("presence of auth bearer %v, want %v", got, want)
				}
				if ok {
					if got, want := len(authBearer), 1; got != want {
						t.Fatalf("got %v auth bearer(s), want %v", got, want)
					}
					if got, want := authBearer[0], tt.wantAuthBearer; got != want {
						t.Errorf("got auth bearer %v, want %v", got, want)
					}
				}

				userAgent, ok := r.Header["User-Agent"]
				if got, want := ok, (tt.wantUserAgent != ""); got != want {
					t.Fatalf("presence of user agent %v, want %v", got, want)
				}
				if ok {
					if got, want := len(userAgent), 1; got != want {
						t.Fatalf("got %v user agent(s), want %v", got, want)
					}
					if got, want := userAgent[0], tt.wantUserAgent; got != want {
						t.Errorf("got user agent %v, want %v", got, want)
					}
				}
			}
		})
	}
}
