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

// HpuInfinibandDevices returns infiniband cards for the accelerators.
func HpuInfinibandDevices() ([]string, error) {
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

// HpuUverbsForIds returns InfiniBand devices corresponding to the accelerator IDs specified in the filter string.
// The filter string is a comma-separated list of device IDs (e.g., "1,2,3").
func HpuUverbsForIDs(visibleDevIDs string) ([]string, error) {
	devs, err := HpuInfinibandDevices()
	if err != nil {
		return nil, err
	}

	devs, err = HpuFilterDevsByIDs(devs, visibleDevIDs)
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

// HpuGetDevIdFromPath extracts the device ID (the last numeric part) from a device path.
func HpuGetDevIDFromPath(devPath string) (string, error) {
	value := strings.IndexFunc(devPath, unicode.IsDigit)
	if value >= 0 && value <= len(devPath) {
		return devPath[value:], nil
	}

	return "", fmt.Errorf("couldn't find device ID in '%s'", devPath)
}

// HpuFilterDevsByIDs returns a filtered devices array based on the filter string.
// The filter string is a comma-separated string of device IDs, e.g., "1,2,3,4".
// A special value of "all" is also supported. An empty filter string yields an empty resulting array.
func HpuFilterDevsByIDs(devs []string, filterIDs string) ([]string, error) {
	sylog.Debugf("Using filter=%s for HPU devices", filterIDs)
	if strings.ToLower(filterIDs) == "all" {
		return devs, nil
	}

	askedIDs := strings.Split(filterIDs, ",")
	sylog.Debugf("Parsed IDs filter: %v", askedIDs)

	// No IDs were requested, returning empty array
	if len(askedIDs) == 0 {
		return askedIDs, nil
	}

	// Actual filtering by device IDs
	var res []string
	for _, dev := range devs {
		if id, err := HpuGetDevIDFromPath(dev); err != nil {
			sylog.Warningf("Unable to parse HPU device path: %s", err)
		} else {
			if slices.Contains(askedIDs, id) {
				res = append(res, dev)
			}
		}
	}

	return res, nil
}

// HpuDevices returns a list of HPU accelerator devices based on the visible device IDs string.
// The string is a comma-separated list of device IDs (e.g., "1,2,3").
// A special value of "all" returns all available devices.
// An empty string yields an empty result.
func HpuDevices(visibleDevIDs string) ([]string, error) {
	sylog.Debugf("Discovering HPU devices")

	devs, err := filepath.Glob("/dev/accel/accel*")
	if err != nil {
		return nil, fmt.Errorf("could not list HPU accelerators: %v", err)
	}

	devs, err = HpuFilterDevsByIDs(devs, visibleDevIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to filter HPU accelerators: %v", err)
	}

	uverbDevs, err := HpuUverbsForIDs(visibleDevIDs)
	if err != nil {
		return nil, fmt.Errorf("could not list uverb devices for HPU accelerators: %v", err)
	}

	devs = append(devs, uverbDevs...)
	return devs, nil
}
