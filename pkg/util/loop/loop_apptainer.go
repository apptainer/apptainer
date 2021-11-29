// Copyright (c) 2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build apptainer_engine
// +build apptainer_engine

package loop

import (
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

func GetMaxLoopDevices() int {
	// if the caller has set the current config use it
	// otherwise parse the default configuration file
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil {
		var err error

		configFile := buildcfg.APPTAINER_CONF_FILE
		cfg, err = apptainerconf.Parse(configFile)
		if err != nil {
			return 256
		}
	}
	return int(cfg.MaxLoopDevices)
}
