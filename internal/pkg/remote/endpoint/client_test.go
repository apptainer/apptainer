// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/registry"

	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

func init() {
	useragent.InitValue("apptainer", "0.1.0")
}

func TestKeyserverClientOpts(t *testing.T) {
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
				URI: SCSDefaultCloudURI,
			},
			uri:           SCSDefaultKeyserverURI,
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverSearchOp,
		},
		{
			name: "Sylabs cloud exclusive KO",
			endpoint: &Config{
				URI:       SCSDefaultCloudURI,
				Exclusive: true,
			},
			uri:           "https://custom.keys",
			expectSuccess: false,
			op:            KeyserverSearchOp,
		},
		{
			name: "Sylabs cloud exclusive OK",
			endpoint: &Config{
				URI:       SCSDefaultCloudURI,
				Exclusive: true,
			},
			uri:           SCSDefaultKeyserverURI,
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverSearchOp,
		},
		{
			name: "Sylabs cloud verify",
			endpoint: &Config{
				URI: SCSDefaultCloudURI,
				Keyservers: []*ServiceConfig{
					{
						URI:  SCSDefaultKeyserverURI,
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
				URI: SCSDefaultCloudURI,
				Keyservers: []*ServiceConfig{
					{
						URI: SCSDefaultKeyserverURI,
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
				URI: SCSDefaultCloudURI,
			},
			uri:           "https://custom.keys",
			expectedOpts:  3,
			expectSuccess: true,
			op:            KeyserverVerifyOp,
		},
		{
			name: "Fake cloud",
			endpoint: &Config{
				URI: "cloud.nonexistent-xxxx-domain.io",
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
func TestRegistryClientConfig(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      *Config
		uri           string
		expectSuccess bool
	}{
		{
			name: "GitHub Container Registry",
			endpoint: &Config{
				URI: DefaultApptainerHost,
			},
			uri:           DefaultRegistryURI,
			expectSuccess: true,
		},
		{
			name: "GitHub Container Registry exclusive KO",
			endpoint: &Config{
				URI:       DefaultApptainerHost,
				Exclusive: true,
			},
			uri:           "https://custom.library",
			expectSuccess: false,
		},
		{
			name: "GitHub Container Registry exclusive OK",
			endpoint: &Config{
				URI:       DefaultApptainerHost,
				Exclusive: true,
			},
			uri:           DefaultRegistryURI,
			expectSuccess: true,
		},
		{
			name: "Custom library",
			endpoint: &Config{
				URI: DefaultApptainerHost,
			},
			uri:           "https://custom.library",
			expectSuccess: true,
		},
		{
			name: "Fake cloud",
			endpoint: &Config{
				URI: "cloud.nonexistent-xxxx-domain.io",
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := tt.endpoint.RegistryClientConfig(tt.uri)
			if nil == config {
				config = &registry.Config{}
			}
			if err != nil && tt.expectSuccess {
				t.Errorf("unexpected error: %s", err)
				t.Errorf(" --- for %s with %v __ %v @ config %v", tt.name, tt, tt.endpoint, config)
			} else if err == nil && !tt.expectSuccess {
				t.Errorf("unexpected success")
				t.Errorf(" --- for %s with %v __ %v @ config %v", tt.name, tt, tt.endpoint, config)
			} else if err != nil && !tt.expectSuccess {
				return
			}
			if config.BaseURL != tt.uri {
				t.Errorf("unexpected uri returned: %s instead of %s", config.BaseURL, tt.uri)
				t.Errorf(" --- for %s with %v __ %v @ config %v", tt.name, tt, tt.endpoint, config)
			} else if config.AuthToken != "" {
				t.Errorf("unexpected token returned: %s", config.AuthToken)
			}
		})
	}
}
