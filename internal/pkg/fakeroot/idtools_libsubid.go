//go:build linux && cgo && libsubid
// +build linux,cgo,libsubid

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
	"sync"
	"unsafe"

	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/opencontainers/runtime-spec/specs-go"
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

var libsubidMutex sync.Mutex

func readSubid(user *user.User, groupMapping bool) ([]*Entry, error) {
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

	libsubidMutex.Lock()
	defer libsubidMutex.Unlock()

	if groupMapping {
		nRanges = C.subid_get_gid_ranges(cUsername, &cRanges)
		if nRanges <= 0 {
			nRanges = C.subid_get_gid_ranges(cuidstr, &cRanges)
		}
	} else {
		nRanges = C.subid_get_uid_ranges(cUsername, &cRanges)
		if nRanges <= 0 {
			nRanges = C.subid_get_uid_ranges(cuidstr, &cRanges)
		}
	}
	if nRanges < 0 {
		return nil, fmt.Errorf("error fetching subid range with libsubid: %v", nRanges)
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

// getIDRange determines ID mappings via libsubid.
func getIDRange(groupMapping bool, uid uint32) (*specs.LinuxIDMapping, error) {
	user, err := user.GetPwUID(uid)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve user with UID %d: %s", uid, err)
	}

	entries, err := readSubid(user, groupMapping)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.invalid {
			continue
		}
		if entry.Count >= validRangeCount {
			return &specs.LinuxIDMapping{
				ContainerID: 1,
				HostID:      entry.Start,
				Size:        entry.Count,
			}, nil
		}
	}
	return nil, fmt.Errorf("no valid mapping entry found for %s (%d)", user.Name, uid)
}

// GetUIDRange determines subUID mappings for the user uid via libsubid.
func GetUIDRange(uid uint32) (*specs.LinuxIDMapping, error) {
	return getIDRange(false, uid)
}

// GetGIDRange determines subGID mappings for the user uid via libsubid.
func GetGIDRange(uid uint32) (*specs.LinuxIDMapping, error) {
	return getIDRange(true, uid)
}
