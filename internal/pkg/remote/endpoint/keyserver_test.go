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
	"reflect"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/remote/credential"
)

func TestAddRemoveKeyserver(t *testing.T) {
	const (
		add    = "add"
		remove = "remove"
	)

	const (
		localhostKeyserver = "http://localhost:11371"
	)

	var testErr error
	token := "test"

	testDefaultCredential := &credential.Config{
		URI:  testKeyserverURI,
		Auth: credential.TokenPrefix + token,
	}

	tests := []struct {
		name           string
		operation      string
		uri            string
		order          uint32
		insecure       bool
		wantErr        bool
		wantKeyservers []*ServiceConfig
	}{
		{
			name:      "Add " + localhostKeyserver,
			operation: add,
			uri:       localhostKeyserver,
			insecure:  true,
			wantErr:   false,
			wantKeyservers: []*ServiceConfig{
				{
					URI:        testKeyserverURI,
					credential: testDefaultCredential,
				},
				{
					URI:      localhostKeyserver,
					External: true,
					Insecure: true,
				},
			},
		},
		{
			name:      "Add hkp://localhost as duplicate",
			operation: add,
			uri:       "hkp://localhost",
			wantErr:   true,
		},
		{
			name:      "Add hkps://localhost with out of order",
			operation: add,
			uri:       "hkp://localhost",
			order:     100,
			wantErr:   true,
		},
		{
			name:      "Prepend hkps://localhost",
			operation: add,
			uri:       "hkps://localhost",
			order:     1,
			wantErr:   false,
			wantKeyservers: []*ServiceConfig{
				{
					URI:      "hkps://localhost",
					External: true,
				},
				{
					URI:        testKeyserverURI,
					credential: testDefaultCredential,
				},
				{
					URI:      localhostKeyserver,
					External: true,
					Insecure: true,
				},
			},
		},
		{
			name:      "Add https://localhost as duplicate",
			operation: add,
			uri:       "https://localhost",
			wantErr:   true,
		},
		{
			name:      "Remove hkps://localhost",
			operation: remove,
			uri:       "hkps://localhost",
			wantErr:   false,
			wantKeyservers: []*ServiceConfig{
				{
					URI:        testKeyserverURI,
					credential: testDefaultCredential,
				},
				{
					URI:      localhostKeyserver,
					External: true,
					Insecure: true,
				},
			},
		},
		{
			name:      "Remove " + testKeyserverURI,
			operation: remove,
			uri:       testKeyserverURI,
			wantErr:   false,
			wantKeyservers: []*ServiceConfig{
				{
					URI:        testKeyserverURI,
					Skip:       true,
					credential: testDefaultCredential,
				},
				{
					URI:      localhostKeyserver,
					External: true,
					Insecure: true,
				},
			},
		},
		{
			name:      "Remove primary " + localhostKeyserver,
			operation: remove,
			uri:       localhostKeyserver,
			wantErr:   true,
		},
		{
			name:      "Add " + testKeyserverURI + " as secondary",
			operation: add,
			uri:       testKeyserverURI,
			order:     2,
			wantErr:   false,
			wantKeyservers: []*ServiceConfig{
				{
					URI:      localhostKeyserver,
					External: true,
					Insecure: true,
				},
				{
					URI:        testKeyserverURI,
					credential: testDefaultCredential,
				},
			},
		},
	}

	ep := &Config{
		URI:   testCloudURI,
		Token: token,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.operation {
			case add:
				testErr = ep.AddKeyserver(tt.uri, tt.order, tt.insecure)
			case remove:
				testErr = ep.RemoveKeyserver(tt.uri)
			default:
				t.Fatalf("unknown test operation")
			}

			if tt.wantErr && testErr == nil {
				t.Errorf("unexpected success during %s", tt.operation)
			} else if !tt.wantErr && testErr != nil {
				t.Errorf("unexpected error during %s: %s", tt.operation, testErr)
			} else if tt.wantKeyservers != nil {
				if !reflect.DeepEqual(tt.wantKeyservers, ep.Keyservers) {
					t.Errorf("unexpected keyservers list")
				}
			}
		})
	}
}
