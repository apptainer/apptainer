// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

//go:build getent_fallback && linux

package user

import (
	"errors"
	"os"
	osuser "os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A fake getent that knows one user and one group that are not in the
// local files, plus entries that must be rejected.
const fakeGetent = `#!/bin/sh
case "$1 $2" in
"passwd 4242420"|"passwd ldapuser")
	echo "ldapuser:*:4242420:4242421:LDAP User:/home/ldapuser:/bin/bash" ;;
"group 4242421"|"group ldapgroup")
	echo "ldapgroup:*:4242421:ldapuser" ;;
"passwd 4242422")
	echo "someoneelse:*:4242499:1:Wrong:/:/bin/sh" ;;
"passwd 4242423")
	echo "malformed:*:4242423" ;;
"passwd root")
	echo "root:x:0:0:not from the passwd file:/nowhere:/bin/false" ;;
"passwd envcheck")
	echo "envcheck:*:4242424:1:${LD_PRELOAD:-clean}:/:/bin/sh" ;;
*)
	exit 2 ;;
esac
`

func setFakeGetent(t *testing.T, allowed bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "getent")
	require.NoError(t, os.WriteFile(path, []byte(fakeGetent), 0o755))
	oldFind, oldAllowed := findGetent, getentAllowed
	findGetent = func() (string, error) { return path, nil }
	getentAllowed = func() bool { return allowed }
	t.Cleanup(func() { findGetent, getentAllowed = oldFind, oldAllowed })
}

func TestGetentUserFallback(t *testing.T) {
	ldapuser := &User{
		Name: "ldapuser", UID: 4242420, GID: 4242421,
		Gecos: "LDAP User", Dir: "/home/ldapuser", Shell: "/bin/bash",
	}

	tests := []struct {
		name    string
		allowed bool
		lookup  func() (*User, error)
		want    *User
		wantErr error
	}{
		{
			name:    "uid only known to getent",
			allowed: true,
			lookup:  func() (*User, error) { return GetPwUID(4242420) },
			want:    ldapuser,
		},
		{
			name:    "name only known to getent",
			allowed: true,
			lookup:  func() (*User, error) { return GetPwNam("ldapuser") },
			want:    ldapuser,
		},
		{
			name:    "not running getent when not allowed",
			allowed: false,
			lookup:  func() (*User, error) { return GetPwUID(4242420) },
			wantErr: osuser.UnknownUserIdError(4242420),
		},
		{
			name:    "unknown to getent too",
			allowed: true,
			lookup:  func() (*User, error) { return GetPwNam("nosuchuser") },
			wantErr: osuser.UnknownUserError("nosuchuser"),
		},
		{
			name:    "entry for a different uid is rejected",
			allowed: true,
			lookup:  func() (*User, error) { return GetPwUID(4242422) },
			wantErr: osuser.UnknownUserIdError(4242422),
		},
		{
			name:    "malformed entry is rejected",
			allowed: true,
			lookup:  func() (*User, error) { return GetPwUID(4242423) },
			wantErr: osuser.UnknownUserIdError(4242423),
		},
		{
			name:    "numeric name is not matched by uid",
			allowed: true,
			lookup:  func() (*User, error) { return GetPwNam("4242420") },
			wantErr: osuser.UnknownUserError("4242420"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFakeGetent(t, tt.allowed)
			got, err := tt.lookup()
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetentGroupFallback(t *testing.T) {
	ldapgroup := &Group{Name: "ldapgroup", GID: 4242421}

	tests := []struct {
		name    string
		lookup  func() (*Group, error)
		want    *Group
		wantErr error
	}{
		{
			name:   "gid only known to getent",
			lookup: func() (*Group, error) { return GetGrGID(4242421) },
			want:   ldapgroup,
		},
		{
			name:   "name only known to getent",
			lookup: func() (*Group, error) { return GetGrNam("ldapgroup") },
			want:   ldapgroup,
		},
		{
			name:    "unknown to getent too",
			lookup:  func() (*Group, error) { return GetGrNam("nosuchgroup") },
			wantErr: osuser.UnknownGroupError("nosuchgroup"),
		},
		{
			name:    "numeric name is not matched by gid",
			lookup:  func() (*Group, error) { return GetGrNam("4242421") },
			wantErr: osuser.UnknownGroupError("4242421"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFakeGetent(t, true)
			got, err := tt.lookup()
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Users found by the normal lookup must not be replaced by getent.
func TestGetentNotUsedForKnownUsers(t *testing.T) {
	setFakeGetent(t, true)
	u, err := GetPwNam("root")
	require.NoError(t, err)
	assert.NotEqual(t, "not from the passwd file", u.Gecos)
}

// getent must run with an empty environment.
func TestGetentEnvironment(t *testing.T) {
	setFakeGetent(t, true)
	t.Setenv("LD_PRELOAD", "/tmp/evil.so")
	u, err := GetPwNam("envcheck")
	require.NoError(t, err)
	assert.Equal(t, "clean", u.Gecos)
}

// Before the configuration is loaded, getent must not be looked for.
func TestGetentBeforeConfig(t *testing.T) {
	setFakeGetent(t, true)
	findGetent = func() (string, error) { return "", errors.New("binary path is not set yet") }
	_, err := GetPwUID(4242420)
	assert.Equal(t, osuser.UnknownUserIdError(4242420), err)
}
