// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package credential

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/util/interactive"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/pkg/sylog"

	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

// loginHandlers contains the registered handlers by scheme.
var loginHandlers = make(map[string]loginHandler)

// loginHandler interface implements login and logout for a specific scheme.
type loginHandler interface {
	login(url *url.URL, username, password string, insecure bool, reqAuthFile string) (*Config, error)
	logout(url *url.URL, reqAuthFile string) error
}

func init() {
	oh := &ociHandler{}
	loginHandlers["oras"] = oh
	loginHandlers["docker"] = oh

	kh := &keyserverHandler{}
	loginHandlers["http"] = kh
	loginHandlers["https"] = kh
}

// ensurePassword ensures password is not empty, if it is, a prompt
// is displayed asking user to provide a password, the entered password
// is then returned by this function. If password is not empty this
// function just return the password provided as argument.
func ensurePassword(password string) (string, error) {
	if password == "" {
		question := "Password / Token: "
		input, err := interactive.AskQuestionNoEcho(question)
		if err != nil {
			return "", fmt.Errorf("failed to read password: %s", err)
		}
		if input == "" {
			return "", fmt.Errorf("a password is required")
		}
		return input, nil
	}
	return password, nil
}

// ociHandler handle login/logout for services with docker:// and oras:// scheme.
type ociHandler struct{}

func (h *ociHandler) login(u *url.URL, username, password string, insecure bool, reqAuthFile string) (*Config, error) {
	if u == nil {
		return nil, fmt.Errorf("URL not provided for login")
	}
	registry := u.Host + u.Path

	if username == "" {
		return nil, fmt.Errorf("Docker/OCI registry requires a username")
	}
	pass, err := ensurePassword(password)
	if err != nil {
		return nil, err
	}

	if err := ociauth.LoginAndStore(registry, username, pass, insecure, reqAuthFile); err != nil {
		return nil, err
	}

	return &Config{
		URI:      u.String(),
		Insecure: insecure,
	}, nil
}

func (h *ociHandler) logout(u *url.URL, reqAuthFile string) error {
	if u == nil {
		return fmt.Errorf("URL not provided for logout")
	}
	registry := u.Host + u.Path

	cf, err := ociauth.ConfigFileFromPath(ociauth.ChooseAuthFile(reqAuthFile))
	if err != nil {
		return fmt.Errorf("while loading existing OCI registry credentials from %q: %w", ociauth.ChooseAuthFile(reqAuthFile), err)
	}

	if _, ok := cf.GetAuthConfigs()[registry]; !ok {
		sylog.Warningf("There is no existing login to registry %q.", registry)
		return nil
	}

	creds := cf.GetCredentialsStore(registry)
	if _, err := creds.Get(registry); err != nil {
		sylog.Warningf("There is no existing login to registry %q.", registry)
		return nil
	}

	if err := creds.Erase(registry); err != nil {
		return fmt.Errorf("while deleting OCI credentials for registry %q: %w", registry, err)
	}

	sylog.Infof("Token removed from %s", cf.Filename)

	return nil
}

// keyserverHandler handle login/logout for keyserver service.
type keyserverHandler struct{}

//nolint:revive
func (h *keyserverHandler) login(u *url.URL, username, password string, insecure bool, reqAuthFile string) (*Config, error) {
	pass, err := ensurePassword(password)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		}
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	if username == "" {
		req.Header.Set("Authorization", TokenPrefix+pass)
	} else {
		req.SetBasicAuth(username, pass)
	}

	auth := req.Header.Get("Authorization")
	req.Header.Set("User-Agent", useragent.Value())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response from server: %s", resp.Status)
	}

	return &Config{
		URI:      u.String(),
		Auth:     auth,
		Insecure: insecure,
	}, nil
}

func (h *keyserverHandler) logout(_ *url.URL, _ string) error {
	return nil
}
