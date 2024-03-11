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
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/docker/distribution/configuration"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/handlers"

	// necessary imports for registry drivers
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	_ "github.com/docker/distribution/registry/storage/driver/middleware/redirect"
	"github.com/sirupsen/logrus"

	"github.com/apptainer/apptainer/internal/pkg/test/tool/exec"
)

const registryConfigTemplate = `
version: 0.1
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: {{.RootDir}}
  delete:
    enabled: false
  redirect:
    disable: true
http:
  addr: 127.0.0.1:5000
  headers:
    X-Content-Type-Options: [nosniff]
auth:
  token:
    service: Authentication
    issuer: E2E
    realm: {{.Realm}}
    rootcertbundle: {{.RootCertBundle}}
`

func StartRegistry(t *testing.T, env TestEnv) string {
	const (
		rootCert = "root.crt"
		rootKey  = "root.key"
		rootPem  = "root.pem"
	)

	ctx := context.Background()

	authListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not setup listener for docker auth server: %s", err)
	}

	certsDir := filepath.Join(env.TestDir, "certs")
	certFile := filepath.Join(certsDir, rootCert)
	keyFile := filepath.Join(certsDir, rootKey)
	pemFile := filepath.Join(certsDir, rootPem)
	if err := os.Mkdir(certsDir, 0o755); err != nil {
		t.Fatalf("could not create %s: %s", certsDir, err)
	}

	if _, err := osexec.LookPath("openssl"); err != nil {
		t.Fatalf("openssl binary is required to be installed on host")
	}

	cmd := exec.Command(
		"openssl",
		"req", "-x509", "-nodes", "-new", "-sha256", "-days", "1024", "-newkey", "rsa:2048",
		"-keyout", keyFile, "-out", pemFile, "-subj", "/C=US/CN=localhost",
	)
	if res := cmd.Run(t); res.Error != nil {
		t.Fatalf("openssl command failed: %s: error output:\n%s", res.Error, res.Stderr())
	}

	cmd = exec.Command(
		"openssl",
		"x509", "-outform", "pem", "-in", pemFile, "-out", certFile,
	)
	if res := cmd.Run(t); res.Error != nil {
		t.Fatalf("openssl command failed: %s: error output:\n%s", res.Error, res.Stderr())
	}

	go func() {
		// for simplicity let this be brutally stopped once test finished
		if err := startAuthServer(authListener, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("failed to start docker auth server: %s", err))
		}
	}()

	regDir := filepath.Join(env.TestDir, "local-registry")
	if err := os.Mkdir(regDir, 0o755); err != nil {
		t.Fatalf("failed to create registry directory %s: %s", regDir, err)
	}

	tmpl, err := template.New("registry.yaml").Parse(registryConfigTemplate)
	if err != nil {
		t.Fatalf("could not create local registry config template: %+v", err)
	}
	data := struct {
		RootDir        string
		Realm          string
		RootCertBundle string
	}{
		RootDir:        regDir,
		Realm:          fmt.Sprintf("http://%s/auth", authListener.Addr().String()),
		RootCertBundle: certFile,
	}

	configBuffer := new(bytes.Buffer)
	if err := tmpl.Execute(configBuffer, data); err != nil {
		t.Fatalf("could not registries.conf template: %+v", err)
	}
	config, err := configuration.Parse(configBuffer)
	if err != nil {
		t.Fatalf("failed to parse local registry configuration: %s", err)
	}

	logrus.SetLevel(logrus.PanicLevel)
	ctx = dcontext.WithLogger(ctx, dcontext.GetLogger(ctx))

	app := handlers.NewApp(ctx, config)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Cache-Control", "no-cache")
				w.WriteHeader(http.StatusOK)
				return
			}
			app.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: httpTimeout,
	}

	registryListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not setup listener for docker registry: %s", err)
	}

	go func() {
		if err := server.Serve(registryListener); err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("failed to start docker local registry: %s", err))
		}
	}()

	_, port, err := net.SplitHostPort(registryListener.Addr().String())
	if err != nil {
		t.Fatalf("failed to retrieve local registry port: %s", err)
	}

	addr := net.JoinHostPort("localhost", port)

	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/", addr))
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			time.Sleep(time.Second)
			continue
		}
		return addr
	}

	t.Fatalf("local registry not reachable")

	return addr
}
