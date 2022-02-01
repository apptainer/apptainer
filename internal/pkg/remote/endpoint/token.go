// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"fmt"
	"net/http"

	"github.com/apptainer/apptainer/internal/pkg/remote/credential"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

// VerifyToken returns an error if a token is not valid against an endpoint.
// If token is provided as an argument, it will verify the provided token.
// If token is "", it will attempt to verify the configured token for the endpoint.
func (config *Config) VerifyToken(token string) (err error) {
	if config.URI == "" {
		return fmt.Errorf("no endpoint URI")
	}

	defer func() {
		if err == nil {
			sylog.Infof("Access Token Verified!")
		}
	}()

	if token == "" {
		token = config.Token
	}

	sp, err := config.GetAllServices()
	if err != nil {
		return err
	}

	ts, ok := sp[Token]
	if !ok || len(ts) == 0 {
		return fmt.Errorf("no authentication service found")
	}

	client := &http.Client{
		Timeout: defaultTimeout,
	}
	req, err := http.NewRequest(http.MethodGet, ts[0].URI()+"/v1/token-status", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", credential.TokenPrefix+token)
	req.Header.Set("User-Agent", useragent.Value())

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to server: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		convStatus, ok := errorCodeMap[res.StatusCode]
		if !ok {
			convStatus = "Unknown"
		}
		return fmt.Errorf("error response from server: %v", convStatus)
	}

	return nil
}
