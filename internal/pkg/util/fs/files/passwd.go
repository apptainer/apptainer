// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package files

import (
	"bufio"
	"fmt"
	"strings"

	pwd "github.com/astromechza/etcpwdparse"
	"github.com/ccoveille/go-safecast/v2"

	"github.com/apptainer/apptainer/internal/pkg/util/fs/layout"
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
func Passwd(path string, home string, uid int, vfs layout.VFS, reader layout.FileReader, customLookup UserGroupLookup) (content []byte, err error) {
	sylog.Verbosef("Checking for template passwd file: %s", path)
	if _, err := vfs.Stat(path); err != nil {
		return content, fmt.Errorf("passwd file doesn't exist in container, not updating")
	}

	sylog.Verbosef("Creating passwd content")
	data, err := reader.ReadFile(path)
	if err != nil {
		return content, fmt.Errorf("error reading passwd file %#v: %v", path, err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Split(bufio.ScanLines)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return content, fmt.Errorf("error reading passwd file %#v: %v", path, err)
	}

	getPwUID := user.GetPwUID
	if customLookup != nil {
		getPwUID = customLookup.GetPwUID
	}

	uid32, err := safecast.Convert[uint32](uid)
	if err != nil {
		return nil, err
	}

	pwInfo, err := getPwUID(uid32)
	if err != nil {
		return content, err
	}

	homeDir := pwInfo.Dir
	if home != "" {
		homeDir = home
	}

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
			gid32, err := safecast.Convert[uint32](entry.Gid())
			if err != nil {
				return nil, err
			}
			// If user already exists in container, change their username, gecos
			// and home dir to those of the original user. Except for uid 0,
			// where the original user is the unprivileged host user (e.g.
			// fakeroot) and not root, so keep the container's existing values.
			name := pwInfo.Name
			gecos := pwInfo.Gecos
			if uid == 0 {
				name = entry.Username()
				gecos = entry.Info()
			}
			lines[i] = makePasswdLine(name, uid32, gid32, gecos, homeDir, entry.Shell())
			break
		}
	}
	if !userExists {
		lines = append(lines, makePasswdLine(pwInfo.Name, pwInfo.UID, pwInfo.GID, pwInfo.Gecos, homeDir, pwInfo.Shell))
	}

	// Add this so that the following strings.Join call will result in text that ends in a newline
	lines = append(lines, "")

	return []byte(strings.Join(lines, "\n")), nil
}

func makePasswdLine(name string, uid uint32, gid uint32, gecos string, homedir string, shell string) string {
	return fmt.Sprintf("%s:x:%d:%d:%s:%s:%s", name, uid, gid, gecos, homedir, shell)
}
