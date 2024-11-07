// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/util/paths"
)

// RocmPaths returns a list of rocm libraries/binaries that should be
// mounted into the container in order to use AMD GPUs
func RocmPaths(configFilePath string) ([]string, []string, error) {
	rocmFiles, err := gpuliblist(configFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read %s: %v", filepath.Base(configFilePath), err)
	}
	// return nil slices to signal that the input was empty
	if len(rocmFiles) == 0 {
		return nil, nil, nil
	}

	libs, bins, _, err := paths.Resolve(rocmFiles)
	return libs, bins, err
}

// RocmDevices returns a list of /dev entries required for ROCm functionality.
func RocmDevices() ([]string, error) {
	// Use same paths as ROCm Docker container documentation.
	// Must bind in all GPU DRI devices, and /dev/kfd device.
	devs := []string{}
	if _, err := os.Stat("/dev/dri"); err == nil {
		devs = append(devs, "/dev/dri")
	}
	if _, err := os.Stat("/dev/kfd"); err == nil {
		devs = append(devs, "/dev/kfd")
	}
	return devs, nil
}
