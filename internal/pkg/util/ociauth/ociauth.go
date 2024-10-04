// Copyright (c) Contributors to the Apptainer project, established as
//
//	Apptainer a Series of LF Projects LLC.
//	For website terms of use, trademark policy, privacy policy and other
//	project policies see https://lfprojects.org/policies
//
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.
package ociauth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	fsutil "github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

const (
	dockerHubRegistry      = "index.docker.io"
	dockerHubRegistryAlias = "docker.io"
	dockerHubAuthKey       = "https://index.docker.io/v1/"
)

type apptainerKeychain struct {
	mu          sync.Mutex
	reqAuthFile string
}

// Resolve implements Keychain.
func (sk *apptainerKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	sk.mu.Lock()
	defer sk.mu.Unlock()

	cf, err := getCredentialsFromFile(ChooseAuthFile(sk.reqAuthFile))
	if err != nil {
		if sk.reqAuthFile != "" {
			// User specifically requested use of an auth file but relevant
			// credentials could not be read from that file; issue warning, but
			// proceed with anonymous authentication.
			sylog.Warningf("Unable to find matching credentials in specified file (%v); proceeding with anonymous authentication.", err)
		}

		// No credentials found; proceed anonymously.
		return authn.Anonymous, nil
	}

	// See:
	// https://github.com/google/ko/issues/90
	// https://github.com/moby/moby/blob/fc01c2b481097a6057bec3cd1ab2d7b4488c50c4/registry/config.go#L397-L404
	var cfg, empty types.AuthConfig
	for _, key := range []string{
		target.String(),
		target.RegistryStr(),
	} {
		// index.docker.io || docker.io => "https://index.docker.io/v1/"
		if key == dockerHubRegistry || key == dockerHubRegistryAlias {
			key = dockerHubAuthKey
		}

		cfg, err = cf.GetAuthConfig(key)
		if err != nil {
			return nil, err
		}
		// cf.GetAuthConfig automatically sets the ServerAddress attribute. Since
		// we don't make use of it, clear the value for a proper "is-empty" test.
		// See: https://github.com/google/go-containerregistry/issues/1510
		cfg.ServerAddress = ""
		if cfg != empty {
			break
		}
	}

	if cfg == empty {
		return authn.Anonymous, nil
	}

	return authn.FromConfig(authn.AuthConfig{
		Username:      cfg.Username,
		Password:      cfg.Password,
		Auth:          cfg.Auth,
		IdentityToken: cfg.IdentityToken,
		RegistryToken: cfg.RegistryToken,
	}), nil
}

// ConfigFileFromPath creates a configfile.Configfile object (part of docker/cli
// API) associated with the auth file at path.
func ConfigFileFromPath(path string) (*configfile.ConfigFile, error) {
	cf := configfile.New(path)
	if fsutil.IsFile(path) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		cf, err = config.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
		cf.Filename = path
	}

	return cf, nil
}

// ChooseAuthFile returns reqAuthFile if it is not empty, or else the default
// location of the OCI registry auth file.
func ChooseAuthFile(reqAuthFile string) string {
	if reqAuthFile != "" {
		return reqAuthFile
	}

	return syfs.SearchDockerConf()
}

func LoginAndStore(registry, username, password string, insecure bool, reqAuthFile string) error {
	if err := checkOCILogin(registry, username, password, insecure); err != nil {
		return err
	}

	cf, err := ConfigFileFromPath(ChooseAuthFile(reqAuthFile))
	if err != nil {
		return fmt.Errorf("while loading existing OCI registry credentials from %q: %w", ChooseAuthFile(reqAuthFile), err)
	}

	creds := cf.GetCredentialsStore(registry)

	// DockerHub requires special logic for historical reasons.
	// index.docker.io || docker.io => "https://index.docker.io/v1/"
	serverAddress := registry
	if serverAddress == dockerHubRegistry || serverAddress == dockerHubRegistryAlias {
		serverAddress = dockerHubAuthKey
	}

	if err := creds.Store(types.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}); err != nil {
		return fmt.Errorf("while trying to store new credentials: %w", err)
	}

	sylog.Infof("Token stored in %s", cf.Filename)

	return nil
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

func getCredentialsFromFile(reqAuthFile string) (*configfile.ConfigFile, error) {
	authFileToUse := ChooseAuthFile(reqAuthFile)
	cf, err := ConfigFileFromPath(authFileToUse)
	if err != nil {
		return nil, fmt.Errorf("while trying to read OCI credentials from file %q: %w", reqAuthFile, err)
	}

	return cf, nil
}

func AuthOptn(ociAuth *authn.AuthConfig, reqAuthFile string) remote.Option {
	if ociAuth != nil {
		return remote.WithAuth(authn.FromConfig(*ociAuth))
	}

	return remote.WithAuthFromKeychain(&apptainerKeychain{reqAuthFile: reqAuthFile})
}
