// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"reflect"
	"testing"
)

func TestHpuGetDevIDFromPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantRes string
		wantErr bool
	}{
		{
			name:    "good-path",
			path:    "/dev/accel/accel1",
			wantRes: "1",
			wantErr: false,
		},
		{
			name:    "big-number",
			path:    "/dev/accel/accel21456",
			wantRes: "21456",
			wantErr: false,
		},
		{
			name:    "invalid-path",
			path:    "/dev/accel/accel",
			wantRes: "",
			wantErr: true,
		},
		{
			name:    "digits-in-the-middle",
			path:    "/dev/0/accel1/accel3",
			wantRes: "3",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := HpuGetDevIDFromPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("HpuGetDevIDFromPath() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if res != tt.wantRes {
				t.Errorf("HpuGetDevIDFromPath() = %v, want = %v", res, tt.wantRes)
			}
		})
	}
}

func TestHpuFilterDevsByIDs(t *testing.T) {
	defaultDevs := []string{
		"/dev/accel/accel0",
		"/dev/accel/accel1",
		"/dev/accel/accel2",
		"/dev/accel/accel3",
		"/dev/accel/accel4",
		"/dev/accel/accel5",
		"/dev/accel/accel6",
		"/dev/accel/accel7",
	}

	tests := []struct {
		name     string
		devs     []string
		filter   string
		wantDevs []string
		wantErr  bool
	}{
		{
			name:     "all",
			devs:     defaultDevs,
			filter:   "all",
			wantDevs: defaultDevs,
			wantErr:  false,
		},
		{
			name:     "all-caps",
			devs:     defaultDevs,
			filter:   "ALL",
			wantDevs: defaultDevs,
			wantErr:  false,
		},
		{
			name:     "proper-filter",
			devs:     defaultDevs,
			filter:   "0,2,3,5,7",
			wantDevs: []string{defaultDevs[0], defaultDevs[2], defaultDevs[3], defaultDevs[5], defaultDevs[7]},
			wantErr:  false,
		},
		{
			name:     "empty-filter",
			devs:     defaultDevs,
			filter:   "",
			wantDevs: nil,
			wantErr:  false,
		},
		{
			name:     "not-exist-filter",
			devs:     defaultDevs,
			filter:   "8,100",
			wantDevs: nil,
			wantErr:  false,
		},
		{
			name:     "not-numbers-filter",
			devs:     defaultDevs,
			filter:   "accel0,boo,foo,",
			wantDevs: nil,
			wantErr:  false,
		},
		{
			name:     "spaces-filter",
			devs:     defaultDevs,
			filter:   " 1, 2 ,3, ",
			wantDevs: []string{defaultDevs[1], defaultDevs[2], defaultDevs[3]},
			wantErr:  false,
		},
		{
			name:     "mixed-filter",
			devs:     defaultDevs,
			filter:   "1,2,3,100,a",
			wantDevs: []string{defaultDevs[1], defaultDevs[2], defaultDevs[3]},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devs, err := HpuFilterDevsByIDs(tt.devs, tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("HpuFilterDevsByIDs() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(devs, tt.wantDevs) {
				t.Errorf("HpuFilterDevsByIDs() = %v, want = %v", devs, tt.wantDevs)
			}
		})
	}
}
