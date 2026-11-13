// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package user

import (
	"errors"
	osuser "os/user"
	"strconv"
	"sync/atomic"
	"syscall"

	"github.com/apptainer/apptainer/pkg/util/namespaces"
)

// User represents a Unix user account information.
type User struct {
	Name  string
	UID   uint32
	GID   uint32
	Gecos string
	Dir   string
	Shell string
}

// Group represents a Unix group information.
type Group struct {
	Name string
	GID  uint32
}

// GetPwUID returns a pointer to User structure associated with user uid.
func GetPwUID(uid uint32) (*User, error) {
	u, err := lookupUnixUid(int(uid))
	if isUnknown(err) {
		return getentUserFallback(strconv.FormatUint(uint64(uid), 10), true, u, err)
	}
	return u, err
}

// GetPwNam returns a pointer to User structure associated with user name.
func GetPwNam(name string) (*User, error) {
	u, err := lookupUser(name)
	if isUnknown(err) {
		return getentUserFallback(name, false, u, err)
	}
	return u, err
}

// GetGrGID returns a pointer to Group structure associated with group gid.
func GetGrGID(gid uint32) (*Group, error) {
	g, err := lookupUnixGid(int(gid))
	if isUnknown(err) {
		return getentGroupFallback(strconv.FormatUint(uint64(gid), 10), true, g, err)
	}
	return g, err
}

// GetGrNam returns a pointer to Group structure associated with group name.
func GetGrNam(name string) (*Group, error) {
	g, err := lookupGroup(name)
	if isUnknown(err) {
		return getentGroupFallback(name, false, g, err)
	}
	return g, err
}

// Current returns a pointer to User structure associated with current user.
func Current() (*User, error) {
	u, err := current()
	if isUnknown(err) {
		return getentUserFallback(strconv.Itoa(syscall.Getuid()), true, u, err)
	}
	return u, err
}

var currentOriginalUser atomic.Pointer[User]

// CurrentOriginal returns a pointer to User structure associated with the
// original current user, if current user is inside a user namespace with a
// custom user mappings, it will returns information about the original user
// otherwise it returns information about the current user
func CurrentOriginal() (*User, error) {
	if u := currentOriginalUser.Load(); u != nil {
		return u, nil
	}
	uid, err := namespaces.HostUID()
	if err != nil {
		return nil, err
	}
	return GetPwUID(uid)
}

// SetCurrentOriginal sets the current original user, used to set the
// original user when user database lookup could fail.
func SetCurrentOriginal(u *User) {
	currentOriginalUser.Store(u)
}

// isUnknown reports whether err means that a user or group does not exist,
// as opposed to the lookup itself failing.
func isUnknown(err error) bool {
	var uid osuser.UnknownUserIdError
	var name osuser.UnknownUserError
	var gid osuser.UnknownGroupIdError
	var group osuser.UnknownGroupError
	return errors.As(err, &uid) || errors.As(err, &name) ||
		errors.As(err, &gid) || errors.As(err, &group)
}
