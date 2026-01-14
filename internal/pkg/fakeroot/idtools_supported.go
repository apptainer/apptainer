//go:build linux && cgo && libsubid

// Portions of this code was adopted from github.com/containers/storage
// Copyright (C) The Linux Foundation and its contributors.
// Original source released under: Apache 2.0 license
// See: https://github.com/containers/storage/blob/main/pkg/idtools/idtools_supported.go
//
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
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/apptainer/apptainer/internal/pkg/util/user"
)

/*
#cgo LDFLAGS: -l subid

#include <shadow/subid.h>
#include <stdlib.h>
#include <stdio.h>

struct subid_range apptainer_get_range(struct subid_range *ranges, int i)
{
	return ranges[i];
}

#if !defined(SUBID_ABI_MAJOR) || (SUBID_ABI_MAJOR < 4)
# define subid_get_uid_ranges get_subuid_ranges
# define subid_get_gid_ranges get_subgid_ranges
#endif
*/
import "C"

func readSubid(user *user.User, isUser bool) ([]*Entry, error) {
	ret := make([]*Entry, 0)
	uidstr := fmt.Sprintf("%d", user.UID)

	if user.Name == "ALL" {
		return nil, errors.New("username ALL not supported")
	}

	cUsername := C.CString(user.Name)
	defer C.free(unsafe.Pointer(cUsername))

	cuidstr := C.CString(uidstr)
	defer C.free(unsafe.Pointer(cuidstr))

	var nRanges C.int
	var cRanges *C.struct_subid_range
	if isUser {
		nRanges = C.subid_get_uid_ranges(cUsername, &cRanges)
		if nRanges <= 0 {
			nRanges = C.subid_get_uid_ranges(cuidstr, &cRanges)
		}
	} else {
		nRanges = C.subid_get_gid_ranges(cUsername, &cRanges)
		if nRanges <= 0 {
			nRanges = C.subid_get_gid_ranges(cuidstr, &cRanges)
		}
	}
	if nRanges < 0 {
		return nil, fmt.Errorf("subid_get_[ug]id_ranges call failed: %v", nRanges)
	}
	defer C.free(unsafe.Pointer(cRanges))

	for i := 0; i < int(nRanges); i++ {
		r := C.apptainer_get_range(cRanges, C.int(i))
		line := fmt.Sprintf("%d:%d:%d", user.UID, r.start, r.count)
		ret = append(
			ret,
			&Entry{
				UID:      user.UID,
				Start:    uint32(r.start),
				Count:    uint32(r.count),
				disabled: false,
				line:     line,
			})
	}
	return ret, nil
}

func readSubuid(user *user.User) ([]*Entry, error) {
	return readSubid(user, true)
}

func readSubgid(user *user.User) ([]*Entry, error) {
	return readSubid(user, false)
}

func (c *Config) getMappingEntries(user *user.User) ([]*Entry, error) {
	entries := make([]*Entry, 0)
	for _, entry := range c.entries {
		if entry.UID == user.UID {
			entries = append(entries, entry)
		}
	}

	var subidEntries []*Entry
	var err error
	if strings.Contains(c.file.Name(), "gid") {
		subidEntries, err = readSubgid(user)
	} else {
		subidEntries, err = readSubuid(user)
	}

	if err != nil {
		return nil, err
	}

	return append(entries, subidEntries...), nil
}
