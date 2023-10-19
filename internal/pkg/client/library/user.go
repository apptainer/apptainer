// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/apptainer/apptainer/pkg/sylog"
	containerclient "github.com/apptainer/container-library-client/client"
)

const (
	userServicePath = "/v1/rbac/users/current"
)

type userOidcMeta struct {
	Secret string `json:"secret"`
}

type userData struct {
	OidcMeta userOidcMeta `json:"oidc_user_meta"`
	Email    string       `json:"email"`
	Realname string       `json:"realname"`
	Username string       `json:"username"`
}

// getUserData retrieves auth service user information from the current remote.
func getUserData(config *containerclient.Config) (*userData, error) {
	client := http.Client{Timeout: 5 * time.Second}
	path := userServicePath
	endPoint := config.BaseURL + path

	req, err := http.NewRequest(http.MethodGet, endPoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating new request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", config.AuthToken))
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		sylog.Debugf("Status Code: %v", res.StatusCode)
		if res.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("must be logged in to retrieve user data")
		}
		if res.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("user endpoint not found")
		}
		return nil, fmt.Errorf("status is not ok: %v", res.StatusCode)
	}

	var ud userData
	err = json.NewDecoder(res.Body).Decode(&ud)
	if err != nil {
		return nil, fmt.Errorf("error decoding json response: %v", err)
	}
	return &ud, nil
}

// GetIdentity returns the username and email for the logged in user.
func GetIdentity(config *containerclient.Config) (user, email string, err error) {
	ud, err := getUserData(config)
	if err != nil {
		return "", "", err
	}
	return ud.Username, ud.Email, nil
}

// GetOCIToken retrieves the OCI registry token for the logged in user.
func GetOCIToken(config *containerclient.Config) (string, error) {
	ud, err := getUserData(config)
	if err != nil {
		return "", err
	}
	if ud.OidcMeta.Secret == "" {
		return "", fmt.Errorf("no token was received from service")
	}
	return ud.OidcMeta.Secret, nil
}
