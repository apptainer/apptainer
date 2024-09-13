// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociplatform

import (
	"reflect"
	"testing"

	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestPlatformFromString(t *testing.T) {
	tests := []struct {
		name    string
		plat    string
		want    *ggcrv1.Platform
		wantErr bool
	}{
		{
			name:    "BadString",
			plat:    "os/arch/variant/extra",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "UnsupportedWindows",
			plat:    "windows/amd64",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "GoodAMD64",
			plat:    "linux/amd64",
			want:    &ggcrv1.Platform{OS: "linux", Architecture: "amd64", Variant: ""},
			wantErr: false,
		},
		{
			name:    "NormalizeARM",
			plat:    "linux/arm",
			want:    &ggcrv1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			wantErr: false,
		},
		{
			name:    "NormalizeARM64/v8",
			plat:    "linux/arm64/v8",
			want:    &ggcrv1.Platform{OS: "linux", Architecture: "arm64", Variant: ""},
			wantErr: false,
		},
		{
			name:    "NormalizeAARCH64",
			plat:    "linux/aarch64",
			want:    &ggcrv1.Platform{OS: "linux", Architecture: "arm64", Variant: ""},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PlatformFromString(tt.plat)
			if (err != nil) != tt.wantErr {
				t.Errorf("PlatformFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PlatformFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}
