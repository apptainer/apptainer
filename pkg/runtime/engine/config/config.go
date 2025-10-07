// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package config

import (
	"encoding/json"

	"github.com/apptainer/apptainer/internal/pkg/metric"
	"github.com/apptainer/apptainer/pkg/plugin"
)

// Common provides the basis for all engine configs. Anything that can not be
// properly described through the OCI config can be stored as a generic JSON []byte.
type Common struct {
	EngineName  string `json:"engineName"`
	ContainerID string `json:"containerID"`
	// EngineConfig is the raw JSON representation of the Engine's underlying config.
	EngineConfig EngineConfig `json:"engineConfig"`

	// PluginConfig is the JSON raw representation of the plugin configurations.
	PluginConfig map[string]json.RawMessage `json:"plugin"`

	// Apptheus connection, this field will be ignored
	ApptheusSocket *metric.Apptheus `json:"-"`
}

// GetPluginConfig retrieves the configuration for the corresponding plugin.
func (c *Common) GetPluginConfig(pl plugin.Plugin, cfg interface{}) error {
	if c.PluginConfig == nil {
		c.PluginConfig = make(map[string]json.RawMessage)
	}
	if raw, found := c.PluginConfig[pl.Name]; found {
		return json.Unmarshal(raw, cfg)
	}
	return nil
}

// SetPluginConfig sets the configuration for the corresponding plugin.
func (c *Common) SetPluginConfig(pl plugin.Plugin, cfg interface{}) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if c.PluginConfig == nil {
		c.PluginConfig = make(map[string]json.RawMessage)
	}
	c.PluginConfig[pl.Name] = raw
	return nil
}

// EngineConfig is a generic interface to represent the implementations of an EngineConfig.
type EngineConfig interface{}
