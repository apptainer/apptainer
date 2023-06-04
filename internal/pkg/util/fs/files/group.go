// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package files

import (
	"fmt"
	"io"
	"os"

	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// Group creates a group template based on content of file provided in path,
// updates content with current user information and returns content
func Group(path string, uid int, gids []int, customLookup UserGroupLookup) (content []byte, err error) {
	duplicate := false
	var groups []int

	sylog.Verbosef("Checking for template group file: %s\n", path)
	if !fs.IsFile(path) {
		return content, fmt.Errorf("group file doesn't exist in container, not updating")
	}

	sylog.Verbosef("Creating group content\n")
	groupFile, err := os.Open(path)
	if err != nil {
		return content, fmt.Errorf("failed to open group file in container: %s", err)
	}
	defer groupFile.Close()

	getPwUID := user.GetPwUID
	getGrGID := user.GetGrGID
	getGroups := os.Getgroups

	if customLookup != nil {
		getPwUID = customLookup.GetPwUID
		getGrGID = customLookup.GetGrGID
		getGroups = customLookup.Getgroups
	}

	pwInfo, err := getPwUID(uint32(uid))
	if err != nil || pwInfo == nil {
		return content, err
	}
	if len(gids) == 0 {
		grInfo, err := getGrGID(pwInfo.GID)
		if err != nil || grInfo == nil {
			return content, err
		}
		groups, err = getGroups()
		if err != nil {
			return content, err
		}
	} else {
		groups = gids
	}
	for _, gid := range groups {
		if gid == int(pwInfo.GID) {
			duplicate = true
			break
		}
	}
	if !duplicate {
		if len(gids) == 0 {
			groups = append(groups, int(pwInfo.GID))
		}
	}
	content, err = io.ReadAll(groupFile)
	if err != nil {
		return content, fmt.Errorf("failed to read group file content in container: %s", err)
	}

	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}

	// https://github.com/apptainer/apptainer/issues/1254
	// only deduplicate newly added groups
	deduplicateStrs := make(map[string]bool)
	for _, gid := range groups {
		grInfo, err := getGrGID(uint32(gid))
		if err != nil || grInfo == nil {
			sylog.Verbosef("Skipping GID %d as group entry doesn't exist.\n", gid)
			continue
		}
		groupLine := fmt.Sprintf("%s:x:%d:%s\n", grInfo.Name, grInfo.GID, pwInfo.Name)
		if _, ok := deduplicateStrs[groupLine]; !ok {
			deduplicateStrs[groupLine] = true
			content = append(content, []byte(groupLine)...)
		}
	}
	return content, nil
}
