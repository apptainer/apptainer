// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.podman.io/image/v5/pkg/sysregistriesv2"
)

// countingRegistry is an in-memory OCI registry that counts image requests.
type countingRegistry struct {
	*httptest.Server
	hits atomic.Int64
}

func newCountingRegistry(t *testing.T) *countingRegistry {
	t.Helper()
	r := &countingRegistry{}
	handler := registry.New()
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v2/" {
			r.hits.Add(1)
		}
		handler.ServeHTTP(w, req)
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *countingRegistry) host() string {
	return strings.TrimPrefix(r.URL, "http://")
}

// pushRandom pushes the same random image to every registry given.
func pushRandom(t *testing.T, repoTag string, regs ...*countingRegistry) {
	t.Helper()
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range regs {
		ref, err := name.ParseReference(r.host()+"/"+repoTag, name.Insecure)
		if err != nil {
			t.Fatal(err)
		}
		if err := remote.Write(ref, img); err != nil {
			t.Fatal(err)
		}
		r.hits.Store(0)
	}
}

// TestRegistriesConf checks that both the digest lookup (used for the cache
// key) and the image pull honor registries.conf, contacting only the mirror
// or rewritten location, never the registry named in the image reference.
func TestRegistriesConf(t *testing.T) {
	tests := []struct {
		name string
		conf string // %[1]s = primary host, %[2]s = mirror host
	}{
		{
			name: "location rewrite",
			conf: `
[[registry]]
prefix = "%[1]s"
location = "%[2]s"
insecure = true
`,
		},
		{
			name: "mirror",
			conf: `
[[registry]]
location = "%[1]s"
insecure = true
[[registry.mirror]]
location = "%[2]s"
insecure = true
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := newCountingRegistry(t)
			mirror := newCountingRegistry(t)
			pushRandom(t, "test/img:v1", primary, mirror)

			conf := filepath.Join(t.TempDir(), "registries.conf")
			if err := os.WriteFile(conf, []byte(fmt.Sprintf(tt.conf, primary.host(), mirror.host())), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CONTAINERS_REGISTRIES_CONF", conf)
			// Parsed configs are cached independently of the env var.
			sysregistriesv2.InvalidateCache()
			t.Cleanup(sysregistriesv2.InvalidateCache)

			ctx := context.Background()
			ref := primary.host() + "/test/img:v1"
			tOpts := &ociimage.TransportOptions{Insecure: true}

			if _, err := ImageDigest(ctx, "docker://"+ref, tOpts); err != nil {
				t.Fatalf("ImageDigest: %v", err)
			}
			if n := primary.hits.Load(); n != 0 {
				t.Errorf("digest lookup made %d requests to the primary registry, want 0", n)
			}

			img, err := ociimage.RegistrySourceSink.Image(ctx, ref, tOpts, nil)
			if err != nil {
				t.Fatalf("pull: %v", err)
			}
			if _, err := img.Manifest(); err != nil {
				t.Fatalf("pull manifest: %v", err)
			}
			if n := primary.hits.Load(); n != 0 {
				t.Errorf("pull made %d requests to the primary registry, want 0", n)
			}
			if mirror.hits.Load() == 0 {
				t.Error("mirror was never contacted")
			}
		})
	}
}
