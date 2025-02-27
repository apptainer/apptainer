//go:build !linux || !libsubid || !cgo
// +build !linux !libsubid !cgo

// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package fakeroot

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// GetIDRange determines UID/GID mappings based on configuration
// file provided in path.
func getIDRange(path string, uid uint32) (*specs.LinuxIDMapping, error) {
	config, err := GetConfig(path, false, getPwNam)
	if err != nil {
		return nil, err
	}
	defer config.Close()

	userinfo, err := getPwUID(uid)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve user with UID %d: %s", uid, err)
	}
	e, err := config.GetUserEntry(userinfo.Name)
	if err != nil {
		return nil, err
	}
	if e.disabled {
		return nil, fmt.Errorf("your fakeroot mapping has been disabled by the administrator")
	}
	return &specs.LinuxIDMapping{
		ContainerID: 1,
		HostID:      e.Start,
		Size:        e.Count,
	}, nil
}

// GetUIDRange determines subUID mappings for the user uid based on the /etc/subuid file.
func GetUIDRange(uid uint32) (*specs.LinuxIDMapping, error) {
	return getIDRange(SubUIDFile, uid)
}

// GetGIDRange determines subUID mappings for the user uid based on the /etc/subgid file.
func GetGIDRange(uid uint32) (*specs.LinuxIDMapping, error) {
	return getIDRange(SubGIDFile, uid)
}
