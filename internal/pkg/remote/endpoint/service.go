// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/remote/credential"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	jsonresp "github.com/sylabs/json-resp"
)

const defaultTimeout = 10 * time.Second

// Default cloud service endpoints.
const (
	DefaultCloudURI     = "cloud.apptainer.org"
	DefaultLibraryURI   = ""
	DefaultKeyserverURI = "https://keys.openpgp.org"
)

// cloud services
const (
	Consent   = "consent"
	Token     = "token"
	Library   = "library"
	Keystore  = "keystore" // alias for keyserver
	Keyserver = "keyserver"
	Builder   = "builder"
)

var errorCodeMap = map[int]string{
	404: "Invalid Credentials",
	500: "Internal Server Error",
}

// ErrStatusNotSupported represents the error returned by
// a service which doesn't support cloud status check.
var ErrStatusNotSupported = errors.New("status not supported")

// Service defines a simple service interface which can be exposed
// to retrieve service URI and check the service status.
type Service interface {
	URI() string
	Status() (string, error)
}

func newService(config *ServiceConfig) Service {
	return &service{cfg: config}
}

type service struct {
	cfg *ServiceConfig
}

// URI returns the service URI.
func (s *service) URI() string {
	return s.cfg.URI
}

// Status checks the service status and returns the version
// of the corresponding service. An ErrStatusNotSupported is
// returned if the service doesn't support this check.
func (s *service) Status() (version string, err error) {
	if s.cfg.External {
		return "", ErrStatusNotSupported
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, s.cfg.URI+"/version", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", useragent.Value())

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to server: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error response from server: %v", res.StatusCode)
	}

	var vRes struct {
		Version string `json:"version"`
	}

	if err := jsonresp.ReadResponse(res.Body, &vRes); err != nil {
		return "", err
	}

	return vRes.Version, nil
}

func (config *Config) GetAllServices() (map[string][]Service, error) {
	if config.services != nil {
		return config.services, nil
	}

	config.services = make(map[string][]Service)

	client := &http.Client{
		Timeout: defaultTimeout,
	}

	epURL, err := config.GetURL()
	if err != nil {
		return nil, err
	}

	configURL := epURL + "/assets/config/config.prod.json"
	req, err := http.NewRequest(http.MethodGet, configURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", useragent.Value())

	cacheReader := getCachedConfig(epURL)
	reader := cacheReader

	if cacheReader == nil {
		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request to server: %s", err)
		} else if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error response from server: %s", err)
		}
		reader = res.Body
	}
	defer reader.Close()

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("while reading response body: %v", err)
	}

	var a map[string]map[string]interface{}

	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("jsonresp: failed to unmarshal response: %v", err)
	}

	if reader != cacheReader {
		updateCachedConfig(epURL, b)
	}

	for k, v := range a {
		s := strings.TrimSuffix(k, "API")
		uri, ok := v["uri"].(string)
		if !ok {
			continue
		}

		serviceConfig := &ServiceConfig{
			URI: uri,
			credential: &credential.Config{
				URI:  uri,
				Auth: credential.TokenPrefix + config.Token,
			},
		}

		if s == Keystore {
			s = Keyserver
		}

		config.services[s] = []Service{
			newService(serviceConfig),
		}
	}

	return config.services, nil
}

// GetServiceURI returns the URI for the service at the specified endpoint
// Examples of services: consent, library, key, token
func (config *Config) GetServiceURI(service string) (string, error) {
	// don't grab remote URI if the endpoint is the
	// default cloud service
	if config.URI == DefaultCloudURI {
		switch service {
		case Library:
			return DefaultLibraryURI, nil
		case Keyserver:
			return DefaultKeyserverURI, nil
		}
	}

	services, err := config.GetAllServices()
	if err != nil {
		return "", err
	}

	s, ok := services[service]
	if !ok || len(s) == 0 {
		return "", fmt.Errorf("%v is not a service at endpoint", service)
	} else if s[0].URI() == "" {
		return "", fmt.Errorf("%v service at endpoint failed to provide URI in response", service)
	}

	return s[0].URI(), nil
}
