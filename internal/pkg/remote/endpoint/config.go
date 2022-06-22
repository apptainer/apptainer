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
	"errors"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/remote/credential"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/gosimple/slug"
)

var cacheDuration = 720 * time.Hour

// DefaultEndpointConfig is the default remote configuration.
var DefaultEndpointConfig = &Config{
	URI:    DefaultCloudURI,
	System: true,
}

var ErrNoURI = errors.New("no URI set for endpoint")

// Config describes a single remote endpoint.
type Config struct {
	URI        string           `yaml:"URI,omitempty"` // hostname/path - no protocol expected
	Token      string           `yaml:"Token,omitempty"`
	System     bool             `yaml:"System"`             // Was this EndPoint set from system config file
	Exclusive  bool             `yaml:"Exclusive"`          // true if the endpoint must be used exclusively
	Insecure   bool             `yaml:"Insecure,omitempty"` // Allow use of http for service discovery
	Keyservers []*ServiceConfig `yaml:"Keyservers,omitempty"`

	// for internal purpose
	credentials []*credential.Config
	services    map[string][]Service
}

func (config *Config) SetCredentials(creds []*credential.Config) {
	config.credentials = creds
}

// GetUrl returns a URL with the correct https or http protocol for the endpoint.
// The protocol depends on whether the endpoint is set 'Insecure'.
func (config *Config) GetURL() (string, error) {
	if config.URI == "" {
		return "", ErrNoURI
	}

	u, err := url.Parse(config.URI)
	if err != nil {
		return "", err
	}

	if config.Insecure {
		u.Scheme = "http"
	} else {
		u.Scheme = "https"
	}

	return u.String(), nil
}

type ServiceConfig struct {
	// for internal purpose
	credential *credential.Config

	URI      string `yaml:"URI"`
	Skip     bool   `yaml:"Skip"`
	External bool   `yaml:"External"`
	Insecure bool   `yaml:"Insecure"`
}

func cacheDir() string {
	cacheDir := syfs.RemoteCacheDir()
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err := os.Mkdir(cacheDir, 0o700); err != nil {
			return ""
		}
	}
	return cacheDir
}

func getCachedConfig(uri string) io.ReadCloser {
	dir := cacheDir()
	if dir == "" {
		return nil
	}
	uriSlug := slug.Make(uri)
	config := filepath.Join(dir, uriSlug+".json")

	fi, err := os.Stat(config)
	if err != nil {
		return nil
	} else if fi.ModTime().Add(cacheDuration).Before(time.Now()) {
		return nil
	}
	rc, err := os.Open(config)
	if err != nil {
		return nil
	}
	return rc
}

func updateCachedConfig(uri string, data []byte) {
	dir := cacheDir()
	if dir == "" {
		return
	}
	config := filepath.Join(dir, uri+".json")
	ioutil.WriteFile(config, data, 0o600)
}
