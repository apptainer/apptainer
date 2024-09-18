//go:build !linux || !libsubid || !cgo
// +build !linux !libsubid !cgo

// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package fakeroot

import (
	"github.com/apptainer/apptainer/internal/pkg/util/user"
)

func (c *Config) getMappingEntries(user *user.User) ([]*Entry, error) {
	entries := make([]*Entry, 0)
	for _, entry := range c.entries {
		if entry.UID == user.UID {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}
