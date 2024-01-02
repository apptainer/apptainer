// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package squashfs

import (
	"os"
	"strconv"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

func getConfig() (*apptainerconf.File, error) {
	// if the caller has set the current config use it
	// otherwise parse the default configuration file
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil {
		sylog.Fatalf("configuration not pre-loaded in squashfs getConfig")
	}
	return cfg, nil
}

// GetPath figures out where the mksquashfs binary is
// and return an error is not available or not usable.
func GetPath() (string, error) {
	return bin.FindBin("mksquashfs")
}

func GetProcs() (uint, error) {
	c, err := getConfig()
	if err != nil {
		return 0, err
	}

	// proc is either "" or the string value in the conf file
	proc := c.MksquashfsProcs

	ompNumThreads := os.Getenv("OMP_NUM_THREADS")
	if ompNumThreads != "" {
		ompNum, err := strconv.Atoi(ompNumThreads)
		if (err == nil) && (ompNum > 0) {
			// OMP_NUM_THREADS can only lower the number
			// of threads to use, never raise it above
			// the admin's config
			if err == nil {
				if proc == 0 || uint(ompNum) < proc {
					proc = uint(ompNum)
				}
			} else {
				// MksquashfsProcs configured to max, but we want fewer
				proc = uint(ompNum)
			}
		}
	}

	return proc, err
}

func GetMem() (string, error) {
	c, err := getConfig()
	if err != nil {
		return "", err
	}
	// mem is either "" or the string value in the conf file
	mem := c.MksquashfsMem

	return mem, err
}
