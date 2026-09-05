// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The GPUs of the fake driver, in PCI bus order, with their minors the
// other way round; a third GPU on a lower bus is excluded.
var (
	testGPUs = []nvidiaGPU{
		{busID: "0000:07:00.0", uuid: "GPU-a3c6697a-c645-295c-72c0-e1e70b5cc9c8", minor: 1},
		{busID: "0000:08:00.0", uuid: "GPU-36413faf-0a4f-722b-0265-f1ccb6e397bf", minor: 0},
	}
	excludedGPU = nvidiaGPU{busID: "0000:06:00.0", uuid: "GPU-8f2ab1c0-5d3e-4b7a-9c61-2e0f7a4d8b13", minor: 2}
)

// fakeDriver writes the driver's GPU information for testGPUs and the
// excluded GPU into a temporary tree, with the DRM entries of the first GPU
// only, and points the package at it. It returns the tree.
func fakeDriver(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	saved := []string{nvidiaProcDir, pciDevicesDir, devDir}
	t.Cleanup(func() { nvidiaProcDir, pciDevicesDir, devDir = saved[0], saved[1], saved[2] })
	nvidiaProcDir = filepath.Join(root, "proc", "driver", "nvidia", "gpus")
	pciDevicesDir = filepath.Join(root, "sys", "bus", "pci", "devices")
	devDir = filepath.Join(root, "dev")

	// listed out of bus order, as a directory listing may be
	for _, gpu := range []nvidiaGPU{testGPUs[1], excludedGPU, testGPUs[0]} {
		dir := filepath.Join(nvidiaProcDir, gpu.busID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Could not create dir: %v", err)
		}
		excluded := "No"
		if gpu == excludedGPU {
			excluded = "Yes"
		}
		info := "Model: \t\t Tesla P100-SXM2-16GB\nIRQ:   \t\t 47\nGPU UUID: \t " + gpu.uuid +
			"\nVideo BIOS: \t ??.??.??.??.??\nBus Type: \t PCIe\nDMA Size: \t 47 bits\nDMA Mask: \t 0x7fffffffffff\nBus Location: \t " + gpu.busID +
			"\nDevice Minor: \t " + string(rune('0'+gpu.minor)) + "\nGPU Excluded:\t " + excluded + "\n"
		if err := os.WriteFile(filepath.Join(dir, "information"), []byte(info), 0o644); err != nil {
			t.Fatalf("Could not create file: %v", err)
		}
	}
	for _, node := range []string{"card1", "renderD128"} {
		if err := os.MkdirAll(filepath.Join(pciDevicesDir, testGPUs[0].busID, "drm", node), 0o755); err != nil {
			t.Fatalf("Could not create dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(devDir, "dri"), 0o755); err != nil {
			t.Fatalf("Could not create dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(devDir, "dri", node), nil, 0o644); err != nil {
			t.Fatalf("Could not create file: %v", err)
		}
	}
	return root
}

func TestNvidiaGPUs(t *testing.T) {
	fakeDriver(t)
	gpus, err := nvidiaGPUs()
	if err != nil {
		t.Fatalf("nvidiaGPUs() error = %v", err)
	}
	if !reflect.DeepEqual(gpus, testGPUs) {
		t.Errorf("nvidiaGPUs() = %+v, expected %+v", gpus, testGPUs)
	}
}

func TestSelectNvidiaGPUs(t *testing.T) {
	for visible, want := range map[string][]nvidiaGPU{
		"all":  testGPUs,
		"":     nil,
		"none": nil,
		"void": nil,
		"1":    testGPUs[1:],
		"1,0":  {testGPUs[1], testGPUs[0]},
		"0,0":  testGPUs[:1],
		"GPU-36413faf-0a4f-722b-0265-f1ccb6e397bf": testGPUs[1:],
		"gpu-36413FAF-0a4f-722b-0265-f1ccb6e397bf": testGPUs[1:],
		"0000:07:00.0":     testGPUs[:1],
		"00000000:07:00.0": testGPUs[:1],
		"0:7:0.0":          testGPUs[:1],
		"0000:07:00.1":     testGPUs[:1],
		"07:00.0":          nil,
		"0000:06:00.0":     nil,
		"0:1":              testGPUs[:1],
		"0:x":              nil,
		"-0":               nil,
		"MIG-36413faf-0a4f-722b-0265-f1ccb6e397bf": nil,
		"2": nil,
	} {
		if got := selectNvidiaGPUs(visible, testGPUs); !reflect.DeepEqual(got, want) {
			t.Errorf("selectNvidiaGPUs(%q) = %+v, expected %+v", visible, got, want)
		}
	}
}

func TestNvidiaDrmDevices(t *testing.T) {
	root := fakeDriver(t)
	nodes := []string{filepath.Join(root, "dev", "dri", "card1"), filepath.Join(root, "dev", "dri", "renderD128")}
	for visible, want := range map[string][]string{
		"all": nodes,
		"0":   nodes,
		"1":   nil,
		"":    nil,
	} {
		if got := NvidiaDrmDevices(visible); !reflect.DeepEqual(got, want) {
			t.Errorf("NvidiaDrmDevices(%q) = %q, expected %q", visible, got, want)
		}
	}
}

// TestNvidiaCreateDevices checks that the helper is asked for each node the
// host lacks, once, with the arguments that create it, through a stand-in
// that creates the nodes as the helper would.
func TestNvidiaCreateDevices(t *testing.T) {
	root := fakeDriver(t)
	if err := os.WriteFile(filepath.Join(devDir, "nvidia0"), nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}
	log := filepath.Join(root, "calls")
	helper := "#!/bin/sh\necho \"$*\" >> " + log + "\ncase \"$*\" in\n" +
		"\"-c=255\") touch " + devDir + "/nvidiactl ;;\n" +
		"-c=*) touch " + devDir + "/nvidia${1#-c=} ;;\n" +
		"\"-u -c=0\") touch " + devDir + "/nvidia-uvm " + devDir + "/nvidia-uvm-tools ;;\n" +
		"-m) touch " + devDir + "/nvidia-modeset ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(binDir, "nvidia-modprobe"), []byte(helper), 0o755); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	NvidiaCreateDevices()

	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("The helper was not called: %v", err)
	}
	want := "-c=255\n-c=1\n-u -c=0\n-m\n"
	if string(calls) != want {
		t.Errorf("nvidia-modprobe was called with %q, expected %q", calls, want)
	}
	entries, _ := os.ReadDir(devDir)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if got, expected := strings.Join(names, " "), "dri nvidia-modeset nvidia-uvm nvidia-uvm-tools nvidia0 nvidia1 nvidiactl"; got != expected {
		t.Errorf("device nodes %q, expected %q", got, expected)
	}
}
