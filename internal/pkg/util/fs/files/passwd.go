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
	"bufio"
	"fmt"
	"os"
	"strings"

	pwd "github.com/astromechza/etcpwdparse"

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
	file, err := os.Open(path)
	if err != nil {
		return content, fmt.Errorf("error opening passwd file %#v for reading: %v", path, err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	file.Close()

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
	userInfo := makePasswdLine(pwInfo.Name, pwInfo.UID, pwInfo.GID, pwInfo.Gecos, homeDir, pwInfo.Shell)

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
			// If user already exists in container, rebuild their passwd info preserving their original shell value
			lines[i] = makePasswdLine(pwInfo.Name, pwInfo.UID, pwInfo.GID, pwInfo.Gecos, homeDir, entry.Shell())
			break
		}
	}
	if !userExists {
		lines = append(lines, userInfo)
	}

	// Add this so that the following strings.Join call will result in text that ends in a newline
	lines = append(lines, "")

	return []byte(strings.Join(lines, "\n")), nil
}

func makePasswdLine(name string, uid uint32, gid uint32, gecos string, homedir string, shell string) string {
	return fmt.Sprintf("%s:x:%d:%d:%s:%s:%s", name, uid, gid, gecos, homedir, shell)
}
