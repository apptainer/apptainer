// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/cgroups"
)

func Test_getBlkioLimits(t *testing.T) {
	tests := []struct {
		name              string
		blkioWeight       int
		blkioWeightDevice []string
		wantBlkio         bool
		wantError         bool
		blkioCheck        func(t *testing.T, b *cgroups.LinuxBlockIO)
	}{
		{
			name:      "None",
			wantBlkio: false,
			wantError: false,
		},
		{
			name:        "GoodWeight",
			blkioWeight: 123,
			wantBlkio:   true,
			wantError:   false,
			blkioCheck: func(t *testing.T, b *cgroups.LinuxBlockIO) {
				if b.Weight == nil {
					t.Fatalf("weight not set")
				}
				if *b.Weight != 123 {
					t.Errorf("expected 123, got %d", *b.Weight)
				}
			},
		},
		{
			name:        "WeightTooLow",
			blkioWeight: 1,
			wantBlkio:   false,
			wantError:   true,
		},
		{
			name:        "WeightTooHigh",
			blkioWeight: 1000000,
			wantBlkio:   false,
			wantError:   true,
		},
		{
			name:              "GoodWeightDevice",
			blkioWeightDevice: []string{"/dev/zero:123"},
			wantBlkio:         true,
			wantError:         false,
			blkioCheck: func(t *testing.T, b *cgroups.LinuxBlockIO) {
				if len(b.WeightDevice) != 1 {
					t.Errorf("expected 1 device entry, got %d", len(b.WeightDevice))
				}
				if b.WeightDevice[0].Major != 1 {
					t.Errorf("expected major 1 , got %d", b.WeightDevice[0].Major)
				}
				if b.WeightDevice[0].Minor != 5 {
					t.Errorf("expected minor 5 , got %d", b.WeightDevice[0].Minor)
				}
				if b.WeightDevice[0].Weight == nil {
					t.Fatalf("weight not set")
				}
				if *b.WeightDevice[0].Weight != 123 {
					t.Errorf("expected weight 123 , got %d", *b.WeightDevice[0].Weight)
				}
			},
		},
		{
			name:              "WeightDeviceBadPath",
			blkioWeightDevice: []string{"/not/a/file:123"},
			wantBlkio:         false,
			wantError:         true,
		},
		{
			name:              "WeightDeviceNotDevice",
			blkioWeightDevice: []string{"/etc/hosts:123"},
			wantBlkio:         false,
			wantError:         true,
		},
		{
			name:              "WeightDeviceWeightTooLow",
			blkioWeightDevice: []string{"/dev/zero:1"},
			wantBlkio:         false,
			wantError:         true,
		},
		{
			name:              "WeightDeviceWeightTooHigh",
			blkioWeightDevice: []string{"/dev/zero:100000"},
			wantBlkio:         false,
			wantError:         true,
		},
		{
			name:              "MultipleWeightDevice",
			blkioWeightDevice: []string{"/dev/zero:123", "/dev/null:123"},
			wantBlkio:         true,
			wantError:         false,
			blkioCheck: func(t *testing.T, b *cgroups.LinuxBlockIO) {
				if len(b.WeightDevice) != 2 {
					t.Errorf("expected 2 device entries, got %d", len(b.WeightDevice))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blkioWeight = tt.blkioWeight
			blkioWeightDevice = tt.blkioWeightDevice

			blkio, err := getBlkioLimits()

			if err != nil && !tt.wantError {
				t.Errorf("unexpected error: %s", err)
			}

			if err == nil && tt.wantError {
				t.Errorf("unexpected success: %s", err)
			}

			if tt.wantBlkio && blkio == nil {
				t.Errorf("expected blkio struct, got nil")
			}

			if !tt.wantBlkio && blkio != nil {
				t.Errorf("expected nil, got %v", blkio)
			}

			if tt.blkioCheck != nil && blkio != nil {
				tt.blkioCheck(t, blkio)
			}
		})
	}
}

func Test_getCpuLimits(t *testing.T) {
	tests := []struct {
		name       string
		cpuShares  int
		cpusetCPUs string
		cpusetMems string
		cpus       string
		wantCPU    bool
		wantError  bool
		cpuCheck   func(t *testing.T, c *cgroups.LinuxCPU)
	}{
		{
			name:      "None",
			wantCPU:   false,
			wantError: false,
		},
		{
			name:      "GoodShares",
			cpuShares: 123,
			wantCPU:   true,
			wantError: false,
			cpuCheck: func(t *testing.T, c *cgroups.LinuxCPU) {
				s := c.Shares
				if s == nil {
					t.Fatalf("shares not set")
				}
				if *s != 123 {
					t.Errorf("expected 123, got %d", *s)
				}
			},
		},
		{
			name:       "GoodCpuset",
			cpusetCPUs: "1-4",
			cpusetMems: "1-4",
			wantCPU:    true,
			wantError:  false,
			cpuCheck: func(t *testing.T, c *cgroups.LinuxCPU) {
				if c.Cpus != "1-4" {
					t.Errorf("expected 1-4, got %s", c.Cpus)
				}
				if c.Mems != "1-4" {
					t.Errorf("expected 1-4, got %s", c.Mems)
				}
			},
		},
		{
			name:      "GoodCpus",
			cpus:      "0.5",
			wantCPU:   true,
			wantError: false,
			cpuCheck: func(t *testing.T, c *cgroups.LinuxCPU) {
				if c.Period == nil {
					t.Fatalf("period not set")
				}
				if *c.Period != 100000 {
					t.Errorf("period should always be 100000 (us), got %d", *c.Period)
				}
				if c.Quota == nil {
					t.Fatalf("quota not set")
				}
				if *c.Quota != 50000 {
					t.Errorf("quota should be 50000 (us), got %d", *c.Quota)
				}
			},
		},
		{
			name:      "CpusInvalid",
			cpus:      "abc",
			wantCPU:   false,
			wantError: true,
		},
		{
			name:      "CpusTooLow",
			cpus:      "0.001",
			wantCPU:   false,
			wantError: true,
		},
		{
			name:      "CpusTooHigh",
			cpus:      strconv.Itoa(runtime.NumCPU() + 1),
			wantCPU:   false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuShares = tt.cpuShares
			cpuSetCPUs = tt.cpusetCPUs
			cpuSetMems = tt.cpusetMems
			cpus = tt.cpus

			cpu, err := getCPULimits()

			if err != nil && !tt.wantError {
				t.Errorf("unexpected error: %s", err)
			}

			if err == nil && tt.wantError {
				t.Errorf("unexpected success: %s", err)
			}

			if tt.wantCPU && cpu == nil {
				t.Errorf("expected cpu struct, got nil")
			}

			if !tt.wantCPU && cpu != nil {
				t.Errorf("expected nil, got %v", cpu)
			}

			if tt.cpuCheck != nil && cpu != nil {
				tt.cpuCheck(t, cpu)
			}
		})
	}
}

func Test_getMemoryLimits(t *testing.T) {
	tests := []struct {
		name              string
		memory            string
		memoryReservation string
		memorySwap        string
		oomKillDisable    bool
		wantMem           bool
		wantError         bool
		memCheck          func(t *testing.T, m *cgroups.LinuxMemory)
	}{
		{
			name:      "None",
			wantMem:   false,
			wantError: false,
		},
		{
			name:      "InvalidMemory",
			memory:    "abc",
			wantMem:   false,
			wantError: true,
		},
		{
			name:      "NegativeMemory",
			memory:    "-1",
			wantMem:   false,
			wantError: true,
		},
		{
			name:      "NumericMemory",
			memory:    "1073741824",
			wantMem:   true,
			wantError: false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Limit == nil {
					t.Fatalf("limit not set")
				}
				if *m.Limit != 1073741824 {
					t.Errorf("expected 1073741824, got %d", *m.Limit)
				}
			},
		},
		{
			name:      "SuffixMemory",
			memory:    "1024M",
			wantMem:   true,
			wantError: false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Limit == nil {
					t.Fatalf("limit not set")
				}
				if *m.Limit != 1073741824 {
					t.Errorf("expected 1073741824, got %d", *m.Limit)
				}
			},
		},
		{
			name:      "InvalidMemoryReservation",
			memory:    "abc",
			wantMem:   false,
			wantError: true,
		},
		{
			name:      "NegativeMemoryReservation",
			memory:    "-1",
			wantMem:   false,
			wantError: true,
		},
		{
			name:              "NumericMemoryReservation",
			memoryReservation: "1073741824",
			wantMem:           true,
			wantError:         false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Reservation == nil {
					t.Fatalf("reservation not set")
				}
				if *m.Reservation != 1073741824 {
					t.Errorf("expected 1073741824, got %d", *m.Reservation)
				}
			},
		},
		{
			name:              "SuffixMemoryReservation",
			memoryReservation: "1024M",
			wantMem:           true,
			wantError:         false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Reservation == nil {
					t.Fatalf("reservation not set")
				}
				if *m.Reservation != 1073741824 {
					t.Errorf("expected 1073741824, got %d", *m.Reservation)
				}
			},
		},
		{
			name:       "NumericMemorySwap",
			memorySwap: "1073741824",
			wantMem:    true,
			wantError:  false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Swap == nil {
					t.Fatalf("swap not set")
				}
				if *m.Swap != 1073741824 {
					t.Errorf("expected 1073741824, got %d", *m.Swap)
				}
			},
		},
		{
			name:       "SuffixMemorySwap",
			memorySwap: "1024M",
			wantMem:    true,
			wantError:  false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Swap == nil {
					t.Fatalf("swap not set")
				}
				if *m.Swap != 1073741824 {
					t.Errorf("expected 1073741824, got %d", *m.Swap)
				}
			},
		},
		{
			name:       "UnlimitedMemorySwap",
			memorySwap: "-1",
			wantMem:    true,
			wantError:  false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.Swap == nil {
					t.Fatalf("swap not set")
				}
				if *m.Swap != -1 {
					t.Errorf("expected -1, got %d", *m.Swap)
				}
			},
		},
		{
			name:           "OomKillDsiable",
			oomKillDisable: true,
			wantMem:        true,
			wantError:      false,
			memCheck: func(t *testing.T, m *cgroups.LinuxMemory) {
				if m.DisableOOMKiller == nil {
					t.Fatalf("DisableOOMKiller not set")
				}
				if *m.DisableOOMKiller != true {
					t.Errorf("DisableOOMKiller not true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory = tt.memory
			memoryReservation = tt.memoryReservation
			memorySwap = tt.memorySwap
			oomKillDisable = tt.oomKillDisable

			mem, err := getMemoryLimits()

			if err != nil && !tt.wantError {
				t.Errorf("unexpected error: %s", err)
			}

			if err == nil && tt.wantError {
				t.Errorf("unexpected success: %s", err)
			}

			if tt.wantMem && mem == nil {
				t.Errorf("expected mem struct, got nil")
			}

			if !tt.wantMem && mem != nil {
				t.Errorf("expected nil, got %v", mem)
			}

			if tt.memCheck != nil && mem != nil {
				tt.memCheck(t, mem)
			}
		})
	}
}

func Test_getPidsLimits(t *testing.T) {
	tests := []struct {
		name      string
		pidsLimit int
		wantPids  bool
		wantError bool
		pidsCheck func(t *testing.T, p *cgroups.LinuxPids)
	}{
		{
			name:      "None",
			wantPids:  false,
			wantError: false,
		},
		{
			name:      "GoodPidsLimit",
			pidsLimit: 123,
			wantPids:  true,
			wantError: false,
			pidsCheck: func(t *testing.T, p *cgroups.LinuxPids) {
				if p.Limit != 123 {
					t.Errorf("expected 123, got %d", p.Limit)
				}
			},
		},
		{
			name:      "UnlimitedPidsLimit",
			pidsLimit: -1,
			wantPids:  true,
			wantError: false,
			pidsCheck: func(t *testing.T, p *cgroups.LinuxPids) {
				if p.Limit != -1 {
					t.Errorf("expected -1, got %d", p.Limit)
				}
			},
		},
		{
			name:      "InvalidPidsLimit",
			pidsLimit: -99,
			wantPids:  false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pidsLimit = tt.pidsLimit

			pids, err := getPidsLimits()

			if err != nil && !tt.wantError {
				t.Errorf("unexpected error: %s", err)
			}

			if err == nil && tt.wantError {
				t.Errorf("unexpected success: %s", err)
			}

			if tt.wantPids && pids == nil {
				t.Errorf("expected cpu struct, got nil")
			}

			if !tt.wantPids && pids != nil {
				t.Errorf("expected nil, got %v", pids)
			}

			if tt.pidsCheck != nil && pids != nil {
				tt.pidsCheck(t, pids)
			}
		})
	}
}
