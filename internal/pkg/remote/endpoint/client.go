// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"fmt"
	"strings"

	registryclient "github.com/apptainer/apptainer/internal/pkg/registry"
	remoteutil "github.com/apptainer/apptainer/internal/pkg/remote/util"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	golog "github.com/go-log/log"
	keyclient "github.com/sylabs/scs-key-client/client"
)

func (ep *Config) KeyserverClientOpts(uri string, op KeyserverOp) ([]keyclient.Option, error) {
	// empty uri means to use the default endpoint
	isDefault := uri == ""

	if err := ep.UpdateKeyserversConfig(); err != nil {
		return nil, err
	}

	var primaryKeyserver *ServiceConfig

	for _, kc := range ep.Keyservers {
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
			keyservers = ep.Keyservers
		} else {
			// use the primary keyserver
			keyservers = []*ServiceConfig{
				primaryKeyserver,
			}
		}
	} else if ep.Exclusive {
		available := make([]string, 0)
		found := false
		for _, kc := range ep.Keyservers {
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

	co := []keyclient.Option{
		keyclient.OptBaseURL(uri),
		keyclient.OptUserAgent(useragent.Value()),
		keyclient.OptHTTPClient(newClient(keyservers, op)),
	}
	return co, nil
}

func (ep *Config) RegistryClientConfig(uri string) (*registryclient.Config, error) {
	// empty uri means to use the default endpoint
	isDefault := uri == ""

	config := &registryclient.Config{
		BaseURL:   uri,
		UserAgent: useragent.Value(),
		Logger:    (golog.Logger)(sylog.DebugLogger{}),
	}

	if isDefault {
		registryURI, err := ep.GetServiceURI(Registry)
		if err != nil {
			return nil, fmt.Errorf("unable to get registry service URI: %v", err)
		}
		config.AuthToken = ep.Token
		config.BaseURL = registryURI
	} else if ep.Exclusive {
		registryURI, err := ep.GetServiceURI(Registry)
		if err != nil {
			return nil, fmt.Errorf("unable to get registry service URI: %v", err)
		}
		if !remoteutil.SameURI(uri, registryURI) {
			return nil, fmt.Errorf(
				"endpoint is set as exclusive by the system administrator: only %q can be used",
				registryURI,
			)
		}
	}

	return config, nil
}
