// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package dmtcp

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/checkpoint"
	"github.com/apptainer/apptainer/internal/pkg/util/paths"
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
	buf, err := ioutil.ReadFile(confPath)
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

	libs, bins, err := paths.Resolve(append(conf.Bins, conf.Libs...))
	if err != nil {
		return nil, nil, err
	}

	var usrBins []string
	for _, bin := range bins {
		usrBin := filepath.Join("/usr/bin", filepath.Base(bin))
		usrBins = append(usrBins, strings.Join([]string{bin, usrBin}, ":"))
	}

	return usrBins, libs, nil
}
