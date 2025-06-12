// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package namespaces

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ccoveille/go-safecast"
)

// IsInsideUserNamespace checks if a process is already running in a
// user namespace and also returns if the process has permissions to use
// setgroups in this user namespace.
func IsInsideUserNamespace(pid int) (bool, bool) {
	// default values returned in case of error
	insideUserNs := false
	setgroupsAllowed := false

	// can fail if the kernel doesn't support user namespace
	r, err := os.Open(fmt.Sprintf("/proc/%d/uid_map", pid))
	if err != nil {
		return insideUserNs, setgroupsAllowed
	}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	// we are interested only by the first line of
	// uid_map which would give us the answer quickly
	// based on the value of size field
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		// trust values returned by procfs
		size, _ := strconv.ParseUint(fields[2], 10, 32)

		// a size of 4294967295 means the process is running
		// in the host user namespace
		if uint32(size) == ^uint32(0) {
			return insideUserNs, setgroupsAllowed
		}

		// process is running inside user namespace
		insideUserNs = true

		// should not fail if open call passed
		d, err := os.ReadFile(fmt.Sprintf("/proc/%d/setgroups", pid))
		if err != nil {
			return insideUserNs, setgroupsAllowed
		}
		setgroupsAllowed = string(d) == "allow\n"
	}

	return insideUserNs, setgroupsAllowed
}

// HostUID attempts to find the original host UID if the current
// process is root running inside a user namespace, and if not it
// simply returns the current UID
func HostUID() (uint32, error) {
	safeUid, err := safecast.ToUint32(os.Getuid())
	if err != nil {
		return 0, fmt.Errorf("failed to convert uid to uint32: %s", err)
	}
	return getHostID("uid", safeUid)
}

// Likewise for HostGID
func HostGID() (uint32, error) {
	safeGid, err := safecast.ToUint32(os.Getgid())
	if err != nil {
		return 0, fmt.Errorf("failed to convert gid to uint32: %s", err)
	}
	return getHostID("gid", safeGid)
}

func getHostID(typ string, currentID uint32) (uint32, error) {
	if currentID != 0 {
		return currentID, nil
	}

	idMap := fmt.Sprintf("/proc/self/%s_map", typ)

	f, err := os.Open(idMap)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("failed to read: %s: %s", idMap, err)
		}
		// user namespace not supported
		return currentID, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		size, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to convert size field %s: %s", fields[2], err)
		}
		// not in a user namespace, use current ID
		if uint32(size) == ^uint32(0) {
			break
		}

		// we are inside a user namespace
		parsedID, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to convert container %s field %s: %s", typ, fields[0], err)
		}
		containerID := uint32(parsedID)
		// we can safely assume that a user won't have two
		// consequent ID and we look if current ID match
		// a 1:1 user mapping
		if size == 1 && currentID == containerID {
			id, err := strconv.ParseUint(fields[1], 10, 32)
			if err != nil {
				return 0, fmt.Errorf("failed to convert host %v field %s: %s", typ, fields[1], err)
			}
			return uint32(id), nil
		}
	}

	// return current ID by default
	return currentID, nil
}

// IsUnprivileged returns true if running as an unprivileged user, even
// if the user id is root inside an unprivileged user namespace; otherwise
// it returns false
func IsUnprivileged() bool {
	if os.Geteuid() != 0 {
		return true
	}
	uid, err := HostUID()
	if err != nil {
		return true
	}
	return uid != 0
}
