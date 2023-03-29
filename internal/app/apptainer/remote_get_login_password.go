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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/apptainer/apptainer/pkg/sylog"
	scslibclient "github.com/apptainer/container-library-client/client"
)

// RemoteGetLoginPassword retrieves cli token from oci library shim
func RemoteGetLoginPassword(config *scslibclient.Config) (string, error) {
	client := http.Client{Timeout: 5 * time.Second}
	path := userServicePath
	endPoint := config.BaseURL + path

	req, err := http.NewRequest(http.MethodGet, endPoint, nil)
	if err != nil {
		return "", fmt.Errorf("error creating new request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", config.AuthToken))
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		sylog.Debugf("Status Code: %v", res.StatusCode)
		if res.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("must be logged in to retrieve token")
		}
		return "", fmt.Errorf("status is not ok: %v", res.StatusCode)
	}

	var ud userData
	err = json.NewDecoder(res.Body).Decode(&ud)
	if err != nil {
		return "", fmt.Errorf("error decoding json response: %v", err)
	}

	if ud.OidcMeta.Secret == "" {
		return "", fmt.Errorf("user does not have cli token set")
	}

	return ud.OidcMeta.Secret, nil
}
