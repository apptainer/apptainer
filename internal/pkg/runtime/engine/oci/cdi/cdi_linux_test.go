// Copyright (c) Contributors to the Apptainer project, established as
// Apptainer a Series of LF Projects LLC.
// For website terms of use, trademark policy, privacy policy and other
// project policies see https://lfprojects.org/policies
//
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cdi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// createTestCDISpecs creates temporary CDI spec files for testing.
// It returns the directory containing the specs.
func createTestCDISpecs(t *testing.T) string {
	tmpDir := t.TempDir()

	// Create a CDI spec for vendor.com/gpu class
	gpuSpec := map[string]interface{}{
		"cdiVersion": "0.5.0",
		"kind":       "vendor.com/gpu",
		"devices": []map[string]interface{}{
			{
				"name": "nvidia0",
				"containerEdits": map[string]interface{}{
					"env": []string{"NVIDIA_VISIBLE_DEVICES=0"},
				},
			},
			{
				"name": "nvidia1",
				"containerEdits": map[string]interface{}{
					"env": []string{"NVIDIA_VISIBLE_DEVICES=1"},
				},
			},
		},
	}

	gpuSpecBytes, err := json.Marshal(gpuSpec)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to marshal CDI gpu spec: %v", err)
	}

	gpuSpecFile := filepath.Join(tmpDir, "vendor.com-gpu.json")
	if err := os.WriteFile(gpuSpecFile, gpuSpecBytes, 0o644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write CDI gpu spec file: %v", err)
	}

	// Create a CDI spec for vendor.com/compute class
	computeSpec := map[string]interface{}{
		"cdiVersion": "0.5.0",
		"kind":       "vendor.com/compute",
		"devices": []map[string]interface{}{
			{
				"name": "cuda",
				"containerEdits": map[string]interface{}{
					"env": []string{"CUDA_HOME=/usr/local/cuda"},
				},
			},
		},
	}

	computeSpecBytes, err := json.Marshal(computeSpec)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to marshal CDI compute spec: %v", err)
	}

	computeSpecFile := filepath.Join(tmpDir, "vendor.com-compute.json")
	if err := os.WriteFile(computeSpecFile, computeSpecBytes, 0o644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write CDI compute spec file: %v", err)
	}

	// Create a CDI spec for vendor.com/device class with various test device names
	deviceSpec := map[string]interface{}{
		"cdiVersion": "0.5.0",
		"kind":       "vendor.com/device",
		"devices": []map[string]interface{}{
			{
				"name": "name",
				"containerEdits": map[string]interface{}{
					"env": []string{"TEST=1"},
				},
			},
			{
				"name": "123test",
				"containerEdits": map[string]interface{}{
					"env": []string{"TEST=2"},
				},
			},
			{
				"name": "test_name",
				"containerEdits": map[string]interface{}{
					"env": []string{"TEST=3"},
				},
			},
			{
				"name": "test-name",
				"containerEdits": map[string]interface{}{
					"env": []string{"TEST=4"},
				},
			},
			{
				"name": "test.name",
				"containerEdits": map[string]interface{}{
					"env": []string{"TEST=5"},
				},
			},
		},
	}

	deviceSpecBytes, err := json.Marshal(deviceSpec)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to marshal CDI device spec: %v", err)
	}

	deviceSpecFile := filepath.Join(tmpDir, "vendor.com-device.json")
	if err := os.WriteFile(deviceSpecFile, deviceSpecBytes, 0o644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write CDI device spec file: %v", err)
	}

	// Create a CDI spec for vendor.io/gpu class
	vendorIOSpec := map[string]interface{}{
		"cdiVersion": "0.5.0",
		"kind":       "vendor.io/gpu",
		"devices": []map[string]interface{}{
			{
				"name": "nvidia0",
				"containerEdits": map[string]interface{}{
					"env": []string{"NVIDIA_VISIBLE_DEVICES=0"},
				},
			},
		},
	}

	vendorIOSpecBytes, err := json.Marshal(vendorIOSpec)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to marshal CDI vendor.io spec: %v", err)
	}

	vendorIOSpecFile := filepath.Join(tmpDir, "vendor.io-gpu.json")
	if err := os.WriteFile(vendorIOSpecFile, vendorIOSpecBytes, 0o644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write CDI vendor.io spec file: %v", err)
	}

	return tmpDir
}

// TestAddCdiDevicesValidNames tests AddCdiDevices with valid device names.
func TestAddCdiDevicesValidNames(t *testing.T) {
	cdiDir := createTestCDISpecs(t)

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
			err := AddCdiDevices(spec, tt.devices, []string{cdiDir})
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
	cdiDir := createTestCDISpecs(t)

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
			err := AddCdiDevices(spec, tt.devices, []string{cdiDir})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCdiDevices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// createTestCDISpecsWithMounts creates temporary CDI spec files with mount specifications.
// It returns the directory containing the specs and a host mount directory.
func createTestCDISpecsWithMounts(t *testing.T) (string, string) {
	tmpDir := t.TempDir()

	// Create a host mount directory
	hostMountDir := filepath.Join(tmpDir, "host-mount")
	if err := os.Mkdir(hostMountDir, 0o755); err != nil {
		t.Fatalf("failed to create host mount directory: %v", err)
	}

	// Create a CDI spec with mount specifications
	mountSpec := map[string]interface{}{
		"cdiVersion": "0.5.0",
		"kind":       "vendor.com/mount",
		"devices": []map[string]interface{}{
			{
				"name": "storage",
				"containerEdits": map[string]interface{}{
					"mounts": []map[string]interface{}{
						{
							"containerPath": "/mnt/storage",
							"hostPath":      hostMountDir,
							"options":       []string{"rw"},
						},
					},
					"env": []string{"STORAGE_MOUNTED=true"},
				},
			},
			{
				"name": "readonly",
				"containerEdits": map[string]interface{}{
					"mounts": []map[string]interface{}{
						{
							"containerPath": "/mnt/readonly",
							"hostPath":      hostMountDir,
							"options":       []string{"ro"},
						},
					},
				},
			},
		},
	}

	mountSpecBytes, err := json.Marshal(mountSpec)
	if err != nil {
		t.Fatalf("failed to marshal CDI mount spec: %v", err)
	}

	mountSpecFile := filepath.Join(tmpDir, "vendor.com-mount.json")
	if err := os.WriteFile(mountSpecFile, mountSpecBytes, 0o644); err != nil {
		t.Fatalf("failed to write CDI mount spec file: %v", err)
	}

	return tmpDir, hostMountDir
}

// TestAddCdiDevicesWithMounts tests AddCdiDevices with mount specifications.
func TestAddCdiDevicesWithMounts(t *testing.T) {
	cdiDir, _ := createTestCDISpecsWithMounts(t)

	tests := []struct {
		name    string
		devices []string
		wantErr bool
	}{
		{
			name:    "device with readwrite mount",
			devices: []string{"vendor.com/mount=storage"},
			wantErr: false,
		},
		{
			name:    "device with readonly mount",
			devices: []string{"vendor.com/mount=readonly"},
			wantErr: false,
		},
		{
			name:    "multiple devices with mounts",
			devices: []string{"vendor.com/mount=storage", "vendor.com/mount=readonly"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &specs.Spec{}
			err := AddCdiDevices(spec, tt.devices, []string{cdiDir})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCdiDevices() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify that mounts were added to the spec if no error occurred
			if !tt.wantErr && err == nil {
				if len(spec.Mounts) == 0 {
					t.Errorf("expected mounts to be added to spec, but got none")
				}
			}
		})
	}
}

// TestAddCdiDevicesWithMountAndDeviceNodes tests AddCdiDevices with both mounts and device nodes.
func TestAddCdiDevicesWithMountAndDeviceNodes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a host mount directory
	hostMountDir := filepath.Join(tmpDir, "host-mount")
	if err := os.Mkdir(hostMountDir, 0o755); err != nil {
		t.Fatalf("failed to create host mount directory: %v", err)
	}

	// Create a CDI spec with both device nodes and mounts
	complexSpec := map[string]interface{}{
		"cdiVersion": "0.5.0",
		"kind":       "vendor.com/complex",
		"devices": []map[string]interface{}{
			{
				"name": "fulldevice",
				"containerEdits": map[string]interface{}{
					"deviceNodes": []map[string]interface{}{
						{
							"hostPath":    "/dev/null",
							"path":        "/dev/null-alias",
							"permissions": "rw",
						},
					},
					"mounts": []map[string]interface{}{
						{
							"containerPath": "/mnt/data",
							"hostPath":      hostMountDir,
							"options":       []string{"rw"},
						},
					},
				},
			},
		},
	}

	complexSpecBytes, err := json.Marshal(complexSpec)
	if err != nil {
		t.Fatalf("failed to marshal complex CDI spec: %v", err)
	}

	complexSpecFile := filepath.Join(tmpDir, "vendor.com-complex.json")
	if err := os.WriteFile(complexSpecFile, complexSpecBytes, 0o644); err != nil {
		t.Fatalf("failed to write complex CDI spec file: %v", err)
	}

	spec := &specs.Spec{}
	err = AddCdiDevices(spec, []string{"vendor.com/complex=fulldevice"}, []string{tmpDir})
	if err != nil {
		t.Errorf("AddCdiDevices() with device nodes and mounts failed: %v", err)
	}

	// Verify that mounts were added
	if len(spec.Mounts) == 0 {
		t.Errorf("expected mounts to be added to spec, but got none")
	}
}
