// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"net/http"
	"net/http/httptest"
	"testing"

	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

func init() {
	useragent.InitValue("apptainer", "0.1.0")
}

const testCloudURI = "cloud.sycloud.io"

var (
	testLibraryURI   string
	testKeyserverURI string
)

func TestKeyserverClientOpts(t *testing.T) {
	// first figure out the testKeyserverURI
	cfg := &Config{
		URI: testCloudURI,
	}
	err := cfg.UpdateKeyserversConfig()
	if err != nil {
		t.Errorf("unexpected error from UpdateKeyserversConfig: %s", err)
	} else if len(cfg.Keyservers) == 0 {
		t.Errorf("no Keyservers found by UpdateKeyserversConfig: %s", err)
	} else {
		testKeyserverURI = cfg.Keyservers[0].URI
	}

	tests := []struct {
		name          string
		endpoint      *Config
		uri           string
		expectedOpts  int
		expectSuccess bool
		op            KeyserverOp
	}{
		{
			name: "Sylabs cloud",
			endpoint: &Config{
				URI: testCloudURI,
			},
			uri:           testKeyserverURI,
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverSearchOp,
		},
		{
			name: "Sylabs cloud exclusive KO",
			endpoint: &Config{
				URI:       testCloudURI,
				Exclusive: true,
			},
			uri:           "https://custom.keys",
			expectSuccess: false,
			op:            KeyserverSearchOp,
		},
		{
			name: "Sylabs cloud exclusive OK",
			endpoint: &Config{
				URI:       testCloudURI,
				Exclusive: true,
			},
			uri:           testKeyserverURI,
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverSearchOp,
		},
		{
			name: "Sylabs cloud verify",
			endpoint: &Config{
				URI: testCloudURI,
				Keyservers: []*ServiceConfig{
					{
						URI:  testKeyserverURI,
						Skip: true,
					},
					{
						URI:      "http://localhost:11371",
						External: true,
					},
				},
			},
			uri:           "",
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverVerifyOp,
		},
		{
			name: "Sylabs cloud search",
			endpoint: &Config{
				URI: testCloudURI,
				Keyservers: []*ServiceConfig{
					{
						URI: testKeyserverURI,
					},
					{
						URI:      "http://localhost:11371",
						External: true,
					},
				},
			},
			uri:           "",
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverSearchOp,
		},
		{
			name: "Custom library",
			endpoint: &Config{
				URI: testCloudURI,
			},
			uri:           "https://custom.keys",
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverVerifyOp,
		},
		{
			name: "Fake cloud",
			endpoint: &Config{
				URI: "cloud.inexistent-xxxx-domain.io",
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			co, err := tt.endpoint.KeyserverClientOpts(tt.uri, tt.op)
			if err != nil && tt.expectSuccess {
				t.Errorf("unexpected error: %s", err)
			} else if err == nil && !tt.expectSuccess {
				t.Errorf("unexpected success for %s", tt.name)
			} else if err != nil && !tt.expectSuccess {
				return
			}
			if len(co) != tt.expectedOpts {
				t.Errorf("unexpected number of options: %v", len(co))
			}
		})
	}
}

//nolint:dupl
func TestLibraryClientConfig(t *testing.T) {
	// first figure out the testLibraryURI
	cfg := &Config{
		URI: testCloudURI,
	}
	libCfg, err := cfg.LibraryClientConfig("")
	if err != nil {
		t.Errorf("unexpected error from LibraryClientConfig: %s", err)
	} else {
		testLibraryURI = libCfg.BaseURL
	}

	tests := []struct {
		name          string
		endpoint      *Config
		uri           string
		expectSuccess bool
	}{
		{
			name: "Sylabs cloud",
			endpoint: &Config{
				URI: testCloudURI,
			},
			uri:           testLibraryURI,
			expectSuccess: true,
		},
		{
			name: "Sylabs cloud exclusive KO",
			endpoint: &Config{
				URI:       testCloudURI,
				Exclusive: true,
			},
			uri:           "https://custom.library",
			expectSuccess: false,
		},
		{
			name: "Sylabs cloud exclusive OK",
			endpoint: &Config{
				URI:       testCloudURI,
				Exclusive: true,
			},
			uri:           testLibraryURI,
			expectSuccess: true,
		},
		{
			name: "Custom library",
			endpoint: &Config{
				URI: testCloudURI,
			},
			uri:           "https://custom.library",
			expectSuccess: true,
		},
		{
			name: "Fake cloud",
			endpoint: &Config{
				URI: "cloud.example.com",
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := tt.endpoint.LibraryClientConfig(tt.uri)
			if err != nil {
				if tt.expectSuccess {
					t.Errorf("unexpected error: %s", err)
				}
			} else {
				if !tt.expectSuccess {
					t.Errorf("unexpected success for %s", tt.name)
				}
				if config.BaseURL != tt.uri {
					t.Errorf("unexpected uri returned: %s instead of %s", config.BaseURL, tt.uri)
				} else if config.AuthToken != "" {
					t.Errorf("unexpected token returned: %s", config.AuthToken)
				}
			}
		})
	}
}

func TestConfig_RegistryURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../../../test/remote/config.example.json")
	}))
	t.Cleanup(srv.Close)

	ep := Config{
		URI:      srv.URL,
		Insecure: true,
	}

	expectRegistry := "https://registry.example.com"
	rURI, err := ep.RegistryURI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if rURI != expectRegistry {
		t.Errorf("expected %q, got %q", expectRegistry, rURI)
	}
}
