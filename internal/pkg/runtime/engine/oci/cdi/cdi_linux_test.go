// Copyright (c) Contributors to the Apptainer project, established as
// Apptainer a Series of LF Projects LLC.
// For website terms of use, trademark policy, privacy policy and other
// project policies see https://lfprojects.org/policies
//
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cdi

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// TestAddCdiDevicesValidNames tests AddCdiDevices with valid device names.
func TestAddCdiDevicesValidNames(t *testing.T) {
	tests := []struct {
		name    string
		devices []string
		wantErr bool
	}{
		{
			name:    "simple vendor and device",
			devices: []string{"vendor.com/device=name"},
			wantErr: false,
		},
		{
			name:    "numeric start in name",
			devices: []string{"vendor.com/device=123test"},
			wantErr: false,
		},
		{
			name:    "underscore in name",
			devices: []string{"vendor.com/device=test_name"},
			wantErr: false,
		},
		{
			name:    "dash in name",
			devices: []string{"vendor.com/device=test-name"},
			wantErr: false,
		},
		{
			name:    "dot in name",
			devices: []string{"vendor.com/device=test.name"},
			wantErr: false,
		},
		{
			name:    "multi-level vendor",
			devices: []string{"vendor.io/gpu=nvidia0"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &specs.Spec{}
			err := AddCdiDevices(spec, tt.devices, []string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCdiDevices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAddCdiDevicesInvalidNames tests AddCdiDevices with invalid device names.
func TestAddCdiDevicesInvalidNames(t *testing.T) {
	tests := []struct {
		name    string
		devices []string
		wantErr bool
	}{
		{
			name:    "missing equals sign",
			devices: []string{"vendor.com/device"},
			wantErr: true,
		},
		{
			name:    "missing vendor",
			devices: []string{"/device=name"},
			wantErr: true,
		},
		{
			name:    "missing device class",
			devices: []string{"vendor.com/=name"},
			wantErr: true,
		},
		{
			name:    "missing device name",
			devices: []string{"vendor.com/device="},
			wantErr: true,
		},
		{
			name:    "empty string",
			devices: []string{""},
			wantErr: true,
		},
		{
			name:    "uppercase in name",
			devices: []string{"vendor.com/device=TestName"},
			wantErr: true,
		},
		{
			name:    "space in name",
			devices: []string{"vendor.com/device=test name"},
			wantErr: true,
		},
		{
			name:    "invalid character @",
			devices: []string{"vendor.com/device=test@name"},
			wantErr: true,
		},
		{
			name:    "invalid character !",
			devices: []string{"vendor.com/device=test!name"},
			wantErr: true,
		},
		{
			name:    "uppercase in vendor",
			devices: []string{"Vendor.com/device=name"},
			wantErr: true,
		},
		{
			name:    "too many slashes",
			devices: []string{"vendor.com/device/extra=name"},
			wantErr: true,
		},
		{
			name:    "colon in device",
			devices: []string{"vendor.com/device:extra=name"},
			wantErr: true,
		},
		{
			name:    "name ending with dash",
			devices: []string{"vendor.com/device=test-"},
			wantErr: true,
		},
		{
			name:    "name starting with dash",
			devices: []string{"vendor.com/device=-test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &specs.Spec{}
			err := AddCdiDevices(spec, tt.devices, []string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCdiDevices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAddCdiDevicesNilSpec tests AddCdiDevices with nil spec.
func TestAddCdiDevicesNilSpec(t *testing.T) {
	devices := []string{"vendor.com/device=name"}
	err := AddCdiDevices(nil, devices, []string{})
	if err == nil {
		t.Error("AddCdiDevices() with nil spec should error, got nil")
	}
}

// TestAddCdiDevicesEmptyDeviceList tests AddCdiDevices with empty device list.
func TestAddCdiDevicesEmptyDeviceList(t *testing.T) {
	spec := &specs.Spec{}
	err := AddCdiDevices(spec, []string{}, []string{})
	if err != nil {
		t.Errorf("AddCdiDevices() with empty list should not error, got %v", err)
	}
}

// TestAddCdiDevicesMultipleDevices tests AddCdiDevices with multiple devices.
func TestAddCdiDevicesMultipleDevices(t *testing.T) {
	tests := []struct {
		name    string
		devices []string
		wantErr bool
	}{
		{
			name:    "two valid devices same class",
			devices: []string{"vendor.com/gpu=nvidia0", "vendor.com/gpu=nvidia1"},
			wantErr: false,
		},
		{
			name:    "two valid devices different classes",
			devices: []string{"vendor.com/gpu=nvidia0", "vendor.com/compute=cuda"},
			wantErr: false,
		},
		{
			name:    "mixed valid and invalid",
			devices: []string{"vendor.com/gpu=nvidia0", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &specs.Spec{}
			err := AddCdiDevices(spec, tt.devices, []string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCdiDevices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
