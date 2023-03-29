// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	scslibclient "github.com/apptainer/container-library-client/client"
	"gotest.tools/v3/assert"
)

const (
	validToken   = "validToken"
	invalidToken = "not valid"
)

func TestRemoteGetLoginPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		authToken string
		jsonResp  string
		shallPass bool
	}{
		{
			name:      "happy path",
			shallPass: true,
			authToken: validToken,
			jsonResp: `{
							"admin_role_in_auth": false,
							"comment": "Onboarded via OIDC provider",
							"creation_time": "2023-02-01T21:37:31.626Z",
							"email": "user@sylabs.io",
							"oidc_user_meta": {
								"creation_time": "2023-02-01T21:37:31.626Z",
								"id": 1,
								"secret": "secretsecretsecret",
								"subiss": "subissidhttps://hydra.se.k3s/",
								"update_time": "2023-02-20T23:26:39.841Z",
								"user_id": 3
							},
							"realname": "sylabs-user",
							"sysadmin_flag": true,
							"update_time": "2023-02-07T18:25:40.732Z",
							"user_id": 3,
							"username": "sylabs-user"
						} `,
			password: "secretsecretsecret",
		},
		{
			name:      "invalid token",
			shallPass: false,
			authToken: invalidToken,
			password:  "",
		},
		{
			name:      "empty json response",
			shallPass: false,
			authToken: validToken,
			password:  "",
			jsonResp:  "{}",
		},
		{
			name:      "invalid json response",
			shallPass: false,
			authToken: validToken,
			password:  "",
			jsonResp:  "random non json text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				isTokenValid := r.Header.Get("Authorization") == "Bearer "+validToken

				if r.URL.Path == userServicePath && isTokenValid {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintln(w, tt.jsonResp)
				} else {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintln(w, "Not found")

				}
			}))
			defer srv.Close()

			config := &scslibclient.Config{
				BaseURL:   srv.URL,
				AuthToken: tt.authToken,
			}
			actual, err := RemoteGetLoginPassword(config)
			assert.Equal(t, actual, tt.password)
			if tt.shallPass == true && err != nil {
				t.Fatalf("valid case failed: %s\n", err)
			}

			if tt.shallPass == false && err == nil {
				t.Fatal("invalid case passed")
			}
		})
	}
}
