// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"fmt"
	"net/http"
	"strings"

	remoteutil "github.com/apptainer/apptainer/internal/pkg/remote/util"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	keyClient "github.com/apptainer/container-key-client/client"
	libClient "github.com/apptainer/container-library-client/client"
	golog "github.com/go-log/log"
)

func (config *Config) KeyserverClientOpts(uri string, op KeyserverOp) ([]keyClient.Option, error) {
	// empty uri means to use the default endpoint
	isDefault := uri == ""

	if err := config.UpdateKeyserversConfig(); err != nil {
		return nil, err
	}

	var primaryKeyserver *ServiceConfig

	for _, kc := range config.Keyservers {
		if kc.Skip {
			continue
		}
		primaryKeyserver = kc
		break
	}

	// shouldn't happen
	if primaryKeyserver == nil {
		return nil, fmt.Errorf("no primary keyserver configured")
	}

	var keyservers []*ServiceConfig

	if isDefault {
		uri = primaryKeyserver.URI

		if op == KeyserverVerifyOp {
			// verify operation can query multiple keyserver, the token
			// is automatically set by the custom client
			keyservers = config.Keyservers
		} else {
			// use the primary keyserver
			keyservers = []*ServiceConfig{
				primaryKeyserver,
			}
		}
	} else if config.Exclusive {
		available := make([]string, 0)
		found := false
		for _, kc := range config.Keyservers {
			if kc.Skip {
				continue
			}
			available = append(available, kc.URI)
			if remoteutil.SameKeyserver(uri, kc.URI) {
				found = true
				break
			}
		}
		if !found {
			list := strings.Join(available, ", ")
			return nil, fmt.Errorf(
				"endpoint is set as exclusive by the system administrator: only %q can be used",
				list,
			)
		}
	} else {
		keyservers = []*ServiceConfig{
			{
				URI:      uri,
				External: true,
			},
		}
	}

	co := []keyClient.Option{
		keyClient.OptBaseURL(uri),
		keyClient.OptUserAgent(useragent.Value()),
		keyClient.OptHTTPClient(newClient(keyservers, op)),
	}
	return co, nil
}

func (config *Config) LibraryClientConfig(uri string) (*libClient.Config, error) {
	// empty uri means to use the default endpoint
	isDefault := uri == ""

	libraryConfig := &libClient.Config{
		BaseURL:   uri,
		UserAgent: useragent.Value(),
		Logger:    (golog.Logger)(sylog.DebugLogger{}),
		// TODO - probably should establish an appropriate client timeout here.
		HTTPClient: &http.Client{},
	}

	if isDefault {
		libURI, err := config.GetServiceURI(Library)
		if err != nil {
			return nil, fmt.Errorf("unable to get library service URI: %v", err)
		}
		libraryConfig.AuthToken = config.Token
		libraryConfig.BaseURL = libURI
	} else if config.Exclusive {
		libURI, err := config.GetServiceURI(Library)
		if err != nil {
			return nil, fmt.Errorf("unable to get library service URI: %v", err)
		}
		if !remoteutil.SameURI(uri, libURI) {
			return nil, fmt.Errorf(
				"endpoint is set as exclusive by the system administrator: only %q can be used",
				libURI,
			)
		}
	}

	return libraryConfig, nil
}

// RegistryURI returns the URI of the backing OCI registry for the library service, associated with ep.
func (config *Config) RegistryURI() (string, error) {
	registryURI, err := config.getServiceConfigVal(Library, RegistryURIConfigKey)
	if err != nil {
		return "", err
	}
	if registryURI == "" {
		return "", fmt.Errorf("library does not provide an OCI registry")
	}
	return registryURI, nil
}
