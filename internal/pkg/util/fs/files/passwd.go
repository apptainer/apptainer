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
	"strings"

	"github.com/revel/cmd/utils"
	pwd "github.com/stat0s2p/etcpwdparse"

	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/apptainer/apptainer/pkg/sylog"
)

type UserGroupLookup interface {
	GetPwUID(uint32) (*user.User, error)
	GetGrGID(uint32) (*user.Group, error)
	Getgroups() ([]int, error)
}

// Passwd creates a passwd template based on content of file provided in path,
// updates content with current user information and returns content.
func Passwd(path string, home string, uid int, customLookup UserGroupLookup) (content []byte, err error) {
	sylog.Verbosef("Checking for template passwd file: %s", path)
	if !fs.IsFile(path) {
		return content, fmt.Errorf("passwd file doesn't exist in container, not updating")
	}

	sylog.Verbosef("Creating passwd content")
	lines, err := utils.ReadLines(path)
	if err != nil {
		return content, fmt.Errorf("failed to read passwd file content in container: %s", err)
	}

	getPwUID := user.GetPwUID
	if customLookup != nil {
		getPwUID = customLookup.GetPwUID
	}

	pwInfo, err := getPwUID(uint32(uid))
	if err != nil {
		return content, err
	}

	homeDir := pwInfo.Dir
	if home != "" {
		homeDir = home
	}
	userInfo := fmt.Sprintf("%s:x:%d:%d:%s:%s:%s\n", pwInfo.Name, pwInfo.UID, pwInfo.GID, pwInfo.Gecos, homeDir, pwInfo.Shell)

	sylog.Verbosef("Creating template passwd file and injecting user data: %s", path)
	userExists := false
	for i, line := range lines {
		if line == "" {
			continue
		}

		entry, err := pwd.ParsePasswdLine(line)
		if err != nil {
			return content, fmt.Errorf("failed to parse this /etc/passwd line in container: %#v (%s)", line, err)
		}
		if entry.Uid() == uid {
			userExists = true
			lines[i] = userInfo
			break
		}
	}
	if !userExists {
		lines = append(lines, userInfo)
	}

	return []byte(strings.Join(lines, "\n")), nil
}
