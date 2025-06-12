// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build apptainer_engine

package loop

import (
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/ccoveille/go-safecast"
)

func GetMaxLoopDevices() (int, error) {
	// if the caller has set the current config use it
	// otherwise parse the default configuration file
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil {
		var err error

		configFile := buildcfg.APPTAINER_CONF_FILE
		cfg, err = apptainerconf.Parse(configFile)
		if err != nil {
			return 256, nil
		}
	}
	return safecast.ToInt(cfg.MaxLoopDevices)
}
