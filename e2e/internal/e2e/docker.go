// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/pkg/errors"
)

const dockerInstanceName = "e2e-docker-instance"

var registrySetup struct {
	sync.Once
	up uint32 // 1 if the registry is running, 0 otherwise
	sync.Mutex
}

// PrepRegistry runs a docker registry and pushes in a busybox image and
// the test image using the oras transport.
// This *MUST* be called before any tests using OCI/instances as it
// temporarily  mounts a shadow instance directory in the test user
// $HOME that will obscure any instances of concurrent tests, causing
// them to fail.
func PrepRegistry(t *testing.T, env TestEnv) {
	// The docker registry container is only available for amd64 and arm
	// See: https://hub.docker.com/_/registry?tab=tags
	// Skip on other architectures
	require.ArchIn(t, []string{"amd64", "arm64"})

	registrySetup.Lock()
	defer registrySetup.Unlock()

	registrySetup.Do(func() {
		t.Log("Preparing docker registry instance.")

		EnsureImage(t, env)

		dockerDefinition := "testdata/Docker_registry.def"
		dockerImage := filepath.Join(env.TestDir, "docker-registry")

		env.RunApptainer(
			t,
			WithProfile(RootProfile),
			WithCommand("build"),
			WithArgs("-s", dockerImage, dockerDefinition),
			ExpectExit(0),
		)

		crt := filepath.Join(dockerImage, "certs/root.crt")
		key := filepath.Join(dockerImage, "certs/root.key")

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("could not setup listener for docker auth server: %s", err)
		}

		go func() {
			// for simplicity let this be brutally stopped once test finished
			if err := startAuthServer(ln, crt, key); err != http.ErrServerClosed {
				t.Errorf("docker auth server error: %s", err)
			}
		}()

		var umountFn func(*testing.T)

		registryAuthRealmEnv := fmt.Sprintf("REGISTRY_AUTH_TOKEN_REALM=http://%s/auth", ln.Addr().String())

		env.RunApptainer(
			t,
			WithProfile(RootProfile),
			WithCommand("instance start"),
			WithArgs("-w", dockerImage, dockerInstanceName),
			WithEnv([]string{registryAuthRealmEnv}),
			PreRun(func(t *testing.T) {
				if os.Getenv("E2E_DOCKER_MIRROR") != "" {
					from := "/root/.config/containers/registries.conf"
					to := filepath.Join(dockerImage, from)

					if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
						t.Fatalf("while creating %s: %s", filepath.Dir(to), err)
					}
					if err := fs.CopyFile(from, to, 0o644); err != nil {
						t.Fatalf("while copying %s to %s: %s", from, to, err)
					}
				}
				umountFn = shadowInstanceDirectory(t, env)
			}),
			PostRun(func(t *testing.T) {
				if umountFn != nil {
					umountFn(t)
				}
			}),
			ExpectExit(0),
		)

		// start script in e2e/testdata/Docker_registry.def will listen
		// on port 5111 once docker registry is up and initialized, so
		// we are trying to connect to this port until we got a response,
		// without any response after 30 seconds we abort tests execution
		// because the start script probably failed
		retry := 0
		for {
			conn, err := net.Dial("tcp", "127.0.0.1:5111")
			err = errors.Wrap(err, "connecting to test endpoint in docker registry container")
			if err == nil {
				conn.Close()
				break
			}
			time.Sleep(100 * time.Millisecond)
			retry++
			if retry == 300 {
				t.Fatalf("docker registry unreachable after 30 seconds: %+v", err)
			}
		}

		atomic.StoreUint32(&registrySetup.up, 1)

		env.RunApptainer(
			t,
			WithProfile(UserProfile),
			WithCommand("push"),
			WithArgs(env.ImagePath, env.OrasTestImage),
			ExpectExit(0),
		)
	})
}

// KillRegistry stop and cleanup docker registry.
func KillRegistry(t *testing.T, env TestEnv) {
	if !atomic.CompareAndSwapUint32(&registrySetup.up, 1, 0) {
		return
	}

	var umountFn func(*testing.T)

	env.RunApptainer(
		t,
		WithProfile(RootProfile),
		WithCommand("instance stop"),
		WithArgs("-s", "KILL", dockerInstanceName),
		PreRun(func(t *testing.T) {
			umountFn = shadowInstanceDirectory(t, env)
		}),
		PostRun(func(t *testing.T) {
			if umountFn != nil {
				umountFn(t)
			}
		}),
		ExpectExit(0),
	)
}

// EnsureRegistry fails the current test if the e2e docker registry is not up
func EnsureRegistry(t *testing.T) {
	// The docker registry container is only available for amd64 and arm
	// See: https://hub.docker.com/_/registry?tab=tags
	// Skip on other architectures
	require.ArchIn(t, []string{"amd64", "arm64"})

	if registrySetup.up != 1 {
		t.Fatalf("Registry instance was not setup. e2e.PrepRegistry must be called before this test.")
	}
}
