// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

package build

import (
	"testing"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBuildLabels(t *testing.T) {
	const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	buildTime := time.Date(2026, time.September, 30, 22, 33, 23, 0, time.FixedZone("CEST", 2*60*60))

	tests := []struct {
		name           string
		noLegacyLabels bool
		reproducible   bool
	}{
		{name: "default"},
		{name: "no legacy labels", noLegacyLabels: true},
		{name: "reproducible", reproducible: true},
		{name: "reproducible without legacy labels", noLegacyLabels: true, reproducible: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &types.Bundle{
				SourceDateEpoch: buildTime,
				Recipe: types.Definition{
					Header: map[string]string{"bootstrap": "docker", "from": "alpine"},
				},
				Opts: types.Options{
					NoLegacyLabels:  tt.noLegacyLabels,
					Reproducible:    tt.reproducible,
					Arch:            "amd64",
					Tag:             "docker.io/library/alpine:latest",
					Digest:          digest,
					Overlay:         true,
					OverlayBaseHash: digest,
				},
			}
			labels := map[string]string{
				"org.label-schema.build-date":              "inherited date",
				"org.label-schema.usage.apptainer.version": "inherited version",
				"org.opencontainers.image.title":           "alpine",
			}
			require.NoError(t, addBuildLabels(labels, b))

			if tt.noLegacyLabels {
				assert.NotContains(t, labels, "org.label-schema.build-date")
				assert.NotContains(t, labels, "org.label-schema.usage.apptainer.version")
			} else {
				assert.Equal(t, "Wednesday_30_September_2026_22:33:23_CEST", labels["org.label-schema.build-date"])
				assert.Equal(t, buildcfg.PACKAGE_VERSION, labels["org.label-schema.usage.apptainer.version"])
			}
			assert.Equal(t, "2026-09-30T20:33:23Z", labels["org.opencontainers.image.created"])
			assert.Equal(t, "1.0", labels["org.label-schema.schema-version"])
			assert.Equal(t, "amd64", labels["org.label-schema.build-arch"])
			assert.Equal(t, "amd64", labels["org.opencontainers.image.architecture"])
			assert.Equal(t, "docker", labels["org.label-schema.usage.singularity.deffile.bootstrap"])
			assert.Equal(t, "alpine", labels["org.label-schema.usage.singularity.deffile.from"])
			assert.Equal(t, b.Opts.Tag, labels["org.opencontainers.image.base.name"])
			assert.Equal(t, digest, labels["org.opencontainers.image.base.digest"])
			assert.Equal(t, digest, labels["org.label-schema.usage.singularity.overlay.base-hash"])
			assert.Equal(t, "alpine", labels["org.opencontainers.image.title"])
		})
	}
}
