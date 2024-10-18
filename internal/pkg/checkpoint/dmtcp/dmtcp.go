// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package dmtcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/checkpoint"
	"github.com/apptainer/apptainer/internal/pkg/util/paths"
	"github.com/apptainer/apptainer/pkg/sylog"
	"gopkg.in/yaml.v2"
)

const (
	containerStatepath = "/.checkpoint"
	portFile           = "coord.port"
	logFile            = "coord.log"
)

const (
	dmtcpPath = "dmtcp"
)

func dmtcpDir() string {
	return filepath.Join(checkpoint.StatePath(), dmtcpPath)
}

type Config struct {
	Bins []string `yaml:"bins"`
	Libs []string `yaml:"libs"`
}

func parseConfig() (*Config, error) {
	confPath := filepath.Join(buildcfg.APPTAINER_CONFDIR, "dmtcp-conf.yaml")
	buf, err := os.ReadFile(confPath)
	if err != nil {
		return nil, err
	}

	var c Config
	err = yaml.Unmarshal(buf, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func GetPaths() ([]string, []string, error) {
	conf, err := parseConfig()
	if err != nil {
		return nil, nil, err
	}

	libs, bins, _, err := paths.Resolve(append(conf.Bins, conf.Libs...))
	if err != nil {
		return nil, nil, err
	}

	usrBins := make([]string, 0, len(bins))
	for _, bin := range bins {
		usrBin := filepath.Join("/usr/bin", filepath.Base(bin))
		usrBins = append(usrBins, strings.Join([]string{bin, usrBin}, ":"))
	}

	return usrBins, libs, nil
}

// QuickInstallationCheck is a quick smoke test to see if dmtcp is installed
// on the host for injection by checking for one of the well known dmtcp
// executables in the PATH. If not found a warning is emitted.
func QuickInstallationCheck() {
	_, err := exec.LookPath("dmtcp_launch")
	if err == nil {
		return
	}

	sylog.Warningf("Unable to locate a dmtcp installation, some functionality may not work as expected. Please ensure a dmtcp installation exists or install it following instructions here: https://github.com/dmtcp/dmtcp/blob/master/INSTALL.md")
}
