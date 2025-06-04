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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/apptainer/apptainer/pkg/sylog"
)

// IntelHpuInfinibandDevices returns infiniband cards for the accelerators.
func IntelHpuInfinibandDevices() ([]string, error) {
	devs, err := filepath.Glob("/sys/class/infiniband/hbl_*")
	if err != nil {
		return nil, fmt.Errorf("could not list infiniband cards for HPU accelerators: %v", err)
	}

	if len(devs) > 0 {
		return devs, nil
	}

	devs, err = filepath.Glob("/sys/class/infiniband/hlib_*")
	if err != nil {
		return nil, fmt.Errorf("unable to list infiniband cards (legacy) for HPU accelerators: %v", err)
	}
	return devs, nil
}

// IntelHpuUverbsForIds returns InfiniBand devices corresponding to the accelerator IDs specified in the filter string.
// The filter string is a comma-separated list of device IDs (e.g., "1,2,3").
func IntelHpuUverbsForIDs(visibleDevIDs string) ([]string, error) {
	devs, err := IntelHpuInfinibandDevices()
	if err != nil {
		return nil, err
	}

	devs, err = IntelHpuFilterDevsByIDs(devs, visibleDevIDs)
	if err != nil {
		return nil, err
	}

	res := []string{}
	for _, dev := range devs {
		// Extract uverb from hlib device
		uverbs, err := os.ReadDir(dev + "/device/infiniband_verbs")
		if err != nil {
			return nil, fmt.Errorf("unable to read hlib directory: %v", err)
		}

		if len(uverbs) == 0 {
			sylog.Warningf("No uverbs devices found for device: %v", dev)
			continue
		}

		uverbDev := "/dev/infiniband/" + uverbs[0].Name()

		if _, err := os.Stat(uverbDev); os.IsNotExist(err) {
			sylog.Warningf("Encountered invalid hlib to uverb mapping: %v -> %v", dev, uverbDev)
			continue
		}

		sylog.Debugf("Found corresponding uverb for hlib: %v -> %v", dev, uverbDev)
		res = append(res, uverbDev)
	}

	return res, nil
}

// IntelHpuGetDevIdFromPath extracts the device ID (the last numeric part) from a device path.
func IntelHpuGetDevIDFromPath(devPath string) (string, error) {
	// Find last non-digit character
	pos := len(devPath)
	for i := pos - 1; i >= 0; i-- {
		if !unicode.IsNumber(rune(devPath[i])) {
			pos = i + 1
			break
		}
	}

	if pos > 0 && pos < len(devPath) {
		return devPath[pos:], nil
	}

	return "", fmt.Errorf("couldn't find device ID in '%s'", devPath)
}

// IntelHpuFilterDevsByIDs returns a filtered devices array based on the filter string.
// The filter string is a comma-separated string of device IDs, e.g., "1,2,3,4".
// A special value of "all" is also supported. An empty filter string yields an empty resulting array.
func IntelHpuFilterDevsByIDs(devs []string, filterIDs string) ([]string, error) {
	sylog.Debugf("Using filter=%s for HPU devices", filterIDs)
	if strings.ToLower(filterIDs) == "all" {
		return devs, nil
	}

	askedIDs := strings.Split(filterIDs, ",")
	for i := range askedIDs {
		askedIDs[i] = strings.TrimSpace(askedIDs[i])
	}
	sylog.Debugf("Parsed IDs filter: %v", askedIDs)

	// No IDs were requested, returning empty array
	if len(askedIDs) == 0 {
		return nil, nil
	}

	// Actual filtering by device IDs
	var res []string
	for _, dev := range devs {
		if id, err := IntelHpuGetDevIDFromPath(dev); err != nil {
			sylog.Warningf("Unable to parse HPU device path: %s", err)
		} else {
			if slices.Contains(askedIDs, id) {
				res = append(res, dev)
			}
		}
	}

	return res, nil
}

// IntelHpuDevices returns a list of HPU accelerator devices based on the visible device IDs string.
// The string is a comma-separated list of device IDs (e.g., "1,2,3").
// A special value of "all" returns all available devices.
// An empty string yields an empty result.
func IntelHpuDevices(visibleDevIDs string) ([]string, error) {
	sylog.Debugf("Discovering HPU devices")

	devs, err := filepath.Glob("/dev/accel/accel*")
	if err != nil {
		return nil, fmt.Errorf("could not list HPU accelerators: %v", err)
	}

	devs, err = IntelHpuFilterDevsByIDs(devs, visibleDevIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to filter HPU accelerators: %v", err)
	}

	uverbDevs, err := IntelHpuUverbsForIDs(visibleDevIDs)
	if err != nil {
		return nil, fmt.Errorf("could not list uverb devices for HPU accelerators: %v", err)
	}

	devs = append(devs, uverbDevs...)
	return devs, nil
}
