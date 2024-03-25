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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/interactive"
	"github.com/apptainer/apptainer/pkg/syfs"

	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// loginHandlers contains the registered handlers by scheme.
var loginHandlers = make(map[string]loginHandler)

// loginHandler interface implements login and logout for a specific scheme.
type loginHandler interface {
	login(url *url.URL, username, password string, insecure bool) (*Config, error)
	logout(url *url.URL) error
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

func (h *ociHandler) login(u *url.URL, username, password string, insecure bool) (*Config, error) {
	if u == nil {
		return nil, fmt.Errorf("URL not provided for login")
	}
	regName := u.Host + u.Path

	if username == "" {
		return nil, fmt.Errorf("Docker/OCI registry requires a username")
	}
	pass, err := ensurePassword(password)
	if err != nil {
		return nil, err
	}

	if err := checkOCILogin(regName, username, pass, insecure); err != nil {
		return nil, err
	}

	ociConfig := syfs.DockerConf()

	cf := configfile.New(syfs.DockerConf())
	if fs.IsFile(ociConfig) {
		f, err := os.Open(ociConfig)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		cf, err = config.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
		cf.Filename = syfs.DockerConf()
	}

	creds := cf.GetCredentialsStore(regName)

	// DockerHub requires special logic for historical reasons.
	serverAddress := regName
	if serverAddress == name.DefaultRegistry {
		serverAddress = authn.DefaultAuthKey
	}

	if err := creds.Store(types.AuthConfig{
		Username:      username,
		Password:      pass,
		ServerAddress: serverAddress,
	}); err != nil {
		return nil, fmt.Errorf("while trying to store new credentials: %w", err)
	}

	return &Config{
		URI:      u.String(),
		Insecure: insecure,
	}, nil
}

func checkOCILogin(regName string, username, password string, insecure bool) error {
	regOpts := []name.Option{}
	if insecure {
		regOpts = []name.Option{name.Insecure}
	}
	reg, err := name.NewRegistry(regName, regOpts...)
	if err != nil {
		return err
	}

	auth := authn.FromConfig(authn.AuthConfig{
		Username: username,
		Password: password,
	})

	// Creating a new transport pings the registry and works through auth flow.
	_, err = transport.NewWithContext(context.TODO(), reg, auth, http.DefaultTransport, nil)
	if err != nil {
		return err
	}

	return nil
}

func (h *ociHandler) logout(u *url.URL) error {
	ociConfig := syfs.DockerConf()
	ociConfigNew := syfs.DockerConf() + ".new"
	cf := configfile.New(syfs.DockerConf())
	if fs.IsFile(ociConfig) {
		f, err := os.Open(ociConfig)
		if err != nil {
			return err
		}
		defer f.Close()
		cf, err = config.LoadFromReader(f)
		if err != nil {
			return err
		}
	}

	registry := u.Host + u.Path
	if _, ok := cf.AuthConfigs[registry]; !ok {
		return fmt.Errorf("%q is not logged in", registry)
	}

	delete(cf.AuthConfigs, registry)

	configData, err := json.Marshal(cf)
	if err != nil {
		return err
	}
	if err := os.WriteFile(ociConfigNew, configData, 0o600); err != nil {
		return err
	}
	return os.Rename(ociConfigNew, ociConfig)
}

// keyserverHandler handle login/logout for keyserver service.
type keyserverHandler struct{}

//nolint:revive
func (h *keyserverHandler) login(u *url.URL, username, password string, insecure bool) (*Config, error) {
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

//nolint:revive
func (h *keyserverHandler) logout(u *url.URL) error {
	return nil
}
