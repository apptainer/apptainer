// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/cgroups"
	"github.com/ccoveille/go-safecast"
	"github.com/docker/go-units"
	"github.com/shopspring/decimal"
	"golang.org/x/sys/unix"
)

// getCgroupsJSON returns any applicable cgroups configuration in JSON serialized format.
// It examines the CLI flags that set limits, and any TOML file set with --apply-cgroups.
func getCgroupsJSON() (string, error) {
	config, err := getFlagLimits()
	if err != nil {
		return "", err
	}

	if config != nil && cgroupsTOMLFile != "" {
		return "", fmt.Errorf("cannot apply a cgroups TOML file while using limit flags")
	}

	if config != nil {
		return config.MarshalJSON()
	}

	if cgroupsTOMLFile != "" {
		config, err := cgroups.LoadConfig(cgroupsTOMLFile)
		if err != nil {
			return "", err
		}
		return config.MarshalJSON()
	}
	return "", nil
}

// getFlagLimits returns a cgroups.Config from the cgroup limits CLI flags.
func getFlagLimits() (*cgroups.Config, error) {
	config := cgroups.Config{}
	configured := false

	blkio, err := getBlkioLimits()
	if err != nil {
		return nil, err
	}
	if blkio != nil {
		config.BlockIO = blkio
		configured = true
	}

	cpu, err := getCPULimits()
	if err != nil {
		return nil, err
	}
	if cpu != nil {
		config.CPU = cpu
		configured = true
	}

	mem, err := getMemoryLimits()
	if err != nil {
		return nil, err
	}
	if mem != nil {
		config.Memory = mem
		configured = true
	}

	pids, err := getPidsLimits()
	if err != nil {
		return nil, err
	}
	if pids != nil {
		config.Pids = pids
		configured = true
	}

	if configured {
		return &config, nil
	}

	return nil, nil
}

// getBlkioLimits handles --blkio* flags, converting values into a LinuxBlockIO structure
func getBlkioLimits() (*cgroups.LinuxBlockIO, error) {
	blkio := cgroups.LinuxBlockIO{}
	configured := false

	if blkioWeight > 0 {
		if blkioWeight < 10 || blkioWeight > 1000 {
			return nil, fmt.Errorf("blkio-weight must be in range 10-1000")
		}
		bw := uint16(blkioWeight)
		blkio.Weight = &bw
		configured = true
	}

	// Format of --blkio-device-weight CLI values is...
	//  <device>:<weight>
	//  /dev/sda:123
	// We need to translate the path into device major and minor numbers.
	if len(blkioWeightDevice) > 0 {
		for _, val := range blkioWeightDevice {
			fields := strings.SplitN(val, ":", 2)
			if len(fields) < 2 {
				return nil, fmt.Errorf("blkio-weight-device specifications must be in <device>:<weight> format")
			}

			major, minor, err := deviceMajorMinor(fields[0])
			if err != nil {
				return nil, fmt.Errorf("while examining device: %w", err)
			}

			weight, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("%s is not a valid device weight: %w", fields[1], err)
			}

			if weight < 10 || weight > 1000 {
				return nil, fmt.Errorf("blkio-device-weight must be in range 10-1000")
			}

			bdw := uint16(weight)
			blkio.WeightDevice = append(blkio.WeightDevice, cgroups.LinuxWeightDevice{
				Major:  major,
				Minor:  minor,
				Weight: &bdw,
			})
		}
		configured = true
	}

	if configured {
		return &blkio, nil
	}

	return nil, nil
}

// getBlkioLimits handles --cpu* flags, converting values into a LinuxCPU structure
func getCPULimits() (*cgroups.LinuxCPU, error) {
	cpu := cgroups.LinuxCPU{}
	configured := false

	// Will be converted to cgroups v2 cpu.weight by manager code
	if cpuShares > 0 {
		cs := uint64(cpuShares)
		cpu.Shares = &cs
		configured = true
	}

	if cpuSetCPUs != "" {
		cpu.Cpus = cpuSetCPUs
		configured = true
	}

	if cpuSetMems != "" {
		cpu.Mems = cpuSetMems
		configured = true
	}

	if cpus != "" {
		// Compute fractional CPU shares in cgroups v1 quota/period form.
		// The manager will convert to cgroups v2 cpu.max

		// https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
		// cpu.cfs_quota_us: the total available run-time within a period (in microseconds)
		// cpu.cfs_period_us: the length of a period (in microseconds)
		// The default values are:
		//    cpu.cfs_period_us=100ms

		// Always use default period of 100ms expressed in us (1e6)
		period := uint64(100 * time.Millisecond / time.Microsecond)

		// Parse cpus values as an arbitrary precision decimal. We will compute
		// quota at 1e9 precision, and allow fractions of a CPU down to 0.01.
		// Lower than this gives an invalid argument when setting cpu.max.
		dCpus, err := decimal.NewFromString(cpus)
		if err != nil {
			return nil, fmt.Errorf("invalid cpus value: %w", err)
		}

		minCPU := decimal.New(1, -2) // 10^-2
		maxCPU := decimal.NewFromInt(int64(runtime.NumCPU()))

		if dCpus.LessThan(minCPU) || dCpus.GreaterThan(maxCPU) {
			return nil, fmt.Errorf("cpus value must be in range %s - %s", minCPU.String(), maxCPU.String())
		}

		nanoCPUs, err := safecast.ToUint64(dCpus.Mul(decimal.NewFromInt(1e9)).IntPart())
		if err != nil {
			return nil, err
		}
		quota, err := safecast.ToInt64(nanoCPUs * period / 1e9)
		if err != nil {
			return nil, err
		}
		cpu.Period = &period
		cpu.Quota = &quota
		configured = true
	}

	if configured {
		return &cpu, nil
	}

	return nil, nil
}

// getMemoryLimits handles --memory* flags, converting values into a LinuxMemory structure
func getMemoryLimits() (*cgroups.LinuxMemory, error) {
	mem := cgroups.LinuxMemory{}
	configured := false

	if memory != "" {
		m, err := units.RAMInBytes(memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory value: %w", err)
		}
		mem.Limit = &m
		configured = true
	}

	if memoryReservation != "" {
		mr, err := units.RAMInBytes(memoryReservation)
		if err != nil {
			return nil, fmt.Errorf("invalid memory-reservation value: %w", err)
		}
		mem.Reservation = &mr
		configured = true
	}

	// -1 is valid here as 'unlimited swap'
	if memorySwap == "-1" {
		ms := int64(-1)
		mem.Swap = &ms
		configured = true
	} else if memorySwap != "" {
		ms, err := units.RAMInBytes(memorySwap)
		if err != nil {
			return nil, fmt.Errorf("invalid memory-swap value: %w", err)
		}
		mem.Swap = &ms
		configured = true
	}

	if oomKillDisable {
		okd := true
		mem.DisableOOMKiller = &okd
		configured = true
	}

	if configured {
		return &mem, nil
	}

	return nil, nil
}

// getPidsLimits handles --pids* flags, converting values into a LinuxPids structure
func getPidsLimits() (*cgroups.LinuxPids, error) {
	pids := cgroups.LinuxPids{}
	configured := false

	if pidsLimit < -1 {
		return nil, fmt.Errorf("invalid pids-limit: %d", pids)
	}

	if pidsLimit != 0 {
		pl := int64(pidsLimit)
		pids.Limit = pl
		configured = true
	}

	if configured {
		return &pids, nil
	}

	return nil, nil
}

// deviceMajorMinor returns major and minor numbers for the device at path
func deviceMajorMinor(path string) (major, minor int64, err error) {
	var stat unix.Stat_t
	err = unix.Lstat(path, &stat)
	if err != nil {
		return -1, -1, err
	}

	if stat.Mode&unix.S_IFBLK != unix.S_IFBLK &&
		stat.Mode&unix.S_IFCHR != unix.S_IFCHR &&
		stat.Mode&unix.S_IFIFO != unix.S_IFIFO {
		return -1, -1, fmt.Errorf("%s is not a device", path)
	}

	// Extra casting to uint64 for stat.Rdev to make sure correct type is set correctly on all archs
	// and avoid failures on mips
	return int64(unix.Major(uint64(stat.Rdev))), int64(unix.Minor(uint64(stat.Rdev))), nil
}
