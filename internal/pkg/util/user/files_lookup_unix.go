// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

// Lookups that read /etc/passwd and /etc/group directly, used instead of
// cgo_lookup_unix.go when cgo is disabled or the osusergo build tag is set.
// Static glibc builds set osusergo, because a statically linked glibc
// crashes when it loads the host's NSS modules for these lookups.

//go:build (aix || darwin || dragonfly || freebsd || (!android && linux) || netbsd || openbsd || solaris) && (!cgo || osusergo)

package user

import (
	"bufio"
	"fmt"
	"os"
	osuser "os/user"
	"strconv"
	"strings"
	"syscall"
)

var (
	passwdFile = "/etc/passwd"
	groupFile  = "/etc/group"
)

// maxLineSize bounds a single passwd or group entry; group entries
// with many members can be long.
const maxLineSize = 1024 * 1024

func current() (*User, error) {
	return lookupUnixUid(syscall.Getuid())
}

func currentGroup() (*Group, error) {
	return lookupUnixGid(syscall.Getgid())
}

func lookupUser(username string) (*User, error) {
	u, err := findUser(func(u *User) bool { return u.Name == username })
	if err != nil {
		return nil, fmt.Errorf("user: lookup username %s: %w", username, err)
	}
	if u == nil {
		return nil, osuser.UnknownUserError(username)
	}
	return u, nil
}

//nolint:revive // same name as in cgo_lookup_unix.go
func lookupUnixUid(uid int) (*User, error) {
	u, err := findUser(func(u *User) bool { return int64(u.UID) == int64(uid) })
	if err != nil {
		return nil, fmt.Errorf("user: lookup userid %d: %w", uid, err)
	}
	if u == nil {
		return nil, osuser.UnknownUserIdError(uid)
	}
	return u, nil
}

func lookupGroup(groupname string) (*Group, error) {
	g, err := findGroup(func(g *Group) bool { return g.Name == groupname })
	if err != nil {
		return nil, fmt.Errorf("user: lookup groupname %s: %w", groupname, err)
	}
	if g == nil {
		return nil, osuser.UnknownGroupError(groupname)
	}
	return g, nil
}

func lookupUnixGid(gid int) (*Group, error) {
	g, err := findGroup(func(g *Group) bool { return int64(g.GID) == int64(gid) })
	if err != nil {
		return nil, fmt.Errorf("user: lookup groupid %d: %w", gid, err)
	}
	if g == nil {
		return nil, osuser.UnknownGroupIdError(strconv.Itoa(gid))
	}
	return g, nil
}

// findUser returns the first passwd entry that match accepts, or nil.
func findUser(match func(*User) bool) (*User, error) {
	var found *User
	err := readEntries(passwdFile, 7, func(f []string) bool {
		uid, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil {
			return false
		}
		gid, err := strconv.ParseUint(f[3], 10, 32)
		if err != nil {
			return false
		}
		u := &User{
			Name:  f[0],
			UID:   uint32(uid),
			GID:   uint32(gid),
			Gecos: f[4],
			Dir:   f[5],
			Shell: f[6],
		}
		if match(u) {
			found = u
			return true
		}
		return false
	})
	return found, err
}

// findGroup returns the first group entry that match accepts, or nil.
func findGroup(match func(*Group) bool) (*Group, error) {
	var found *Group
	err := readEntries(groupFile, 4, func(f []string) bool {
		gid, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil {
			return false
		}
		g := &Group{Name: f[0], GID: uint32(gid)}
		if match(g) {
			found = g
			return true
		}
		return false
	})
	return found, err
}

// readEntries calls fn with the colon-separated fields of each entry in
// path that has exactly nfields fields, until fn returns true. Blank
// lines, comments and NIS compat entries starting with + or - are
// skipped, as in Go's os/user.
func readEntries(path string, nfields int, fn func([]string) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 4096), maxLineSize)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' || line[0] == '+' || line[0] == '-' {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != nfields {
			continue
		}
		if fn(fields) {
			return nil
		}
	}
	return s.Err()
}
