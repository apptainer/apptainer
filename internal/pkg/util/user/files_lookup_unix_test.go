// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

//go:build (aix || darwin || dragonfly || freebsd || (!android && linux) || netbsd || openbsd || solaris) && (!cgo || osusergo)

package user

import (
	"os"
	osuser "os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:gosec // passwd file fixture, not credentials
const testPasswd = `# comment
+nisuser::::::

root:x:0:0:root:/root:/bin/bash
bad:x:notanumber:0:bad:/:/bin/sh
short:x:5:5
alice:x:1000:1000:Alice A,,,:/home/alice:/bin/zsh
dup:x:1000:1000:Duplicate:/home/dup:/bin/sh
`

const testGroup = `# comment
-nisgroup:::
root:x:0:
bad:x:notanumber:
wheel:x:10:alice,bob
alice:x:1000:
`

func setTestFiles(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "passwd")
	g := filepath.Join(dir, "group")
	require.NoError(t, os.WriteFile(p, []byte(testPasswd), 0o644))
	require.NoError(t, os.WriteFile(g, []byte(testGroup), 0o644))
	oldPasswd, oldGroup := passwdFile, groupFile
	passwdFile, groupFile = p, g
	t.Cleanup(func() { passwdFile, groupFile = oldPasswd, oldGroup })
}

func TestFilesLookupUser(t *testing.T) {
	setTestFiles(t)
	alice := &User{
		Name: "alice", UID: 1000, GID: 1000,
		Gecos: "Alice A,,,", Dir: "/home/alice", Shell: "/bin/zsh",
	}

	tests := []struct {
		name    string
		lookup  func() (*User, error)
		want    *User
		wantErr error
	}{
		{
			name:   "by name",
			lookup: func() (*User, error) { return lookupUser("alice") },
			want:   alice,
		},
		{
			name:   "by uid returns first match",
			lookup: func() (*User, error) { return lookupUnixUid(1000) },
			want:   alice,
		},
		{
			name:   "root",
			lookup: func() (*User, error) { return lookupUnixUid(0) },
			want:   &User{Name: "root", Gecos: "root", Dir: "/root", Shell: "/bin/bash"},
		},
		{
			name:    "unknown name",
			lookup:  func() (*User, error) { return lookupUser("nobody") },
			wantErr: osuser.UnknownUserError("nobody"),
		},
		{
			name:    "unknown uid",
			lookup:  func() (*User, error) { return lookupUnixUid(4242) },
			wantErr: osuser.UnknownUserIdError(4242),
		},
		{
			name:    "malformed entries are skipped",
			lookup:  func() (*User, error) { return lookupUser("short") },
			wantErr: osuser.UnknownUserError("short"),
		},
		{
			name:    "NIS entries are skipped",
			lookup:  func() (*User, error) { return lookupUser("+nisuser") },
			wantErr: osuser.UnknownUserError("+nisuser"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestFilesLookupGroup(t *testing.T) {
	setTestFiles(t)

	tests := []struct {
		name    string
		lookup  func() (*Group, error)
		want    *Group
		wantErr error
	}{
		{
			name:   "by name",
			lookup: func() (*Group, error) { return lookupGroup("wheel") },
			want:   &Group{Name: "wheel", GID: 10},
		},
		{
			name:   "by gid",
			lookup: func() (*Group, error) { return lookupUnixGid(1000) },
			want:   &Group{Name: "alice", GID: 1000},
		},
		{
			name:    "unknown name",
			lookup:  func() (*Group, error) { return lookupGroup("staff") },
			wantErr: osuser.UnknownGroupError("staff"),
		},
		{
			name:    "unknown gid",
			lookup:  func() (*Group, error) { return lookupUnixGid(4242) },
			wantErr: osuser.UnknownGroupIdError("4242"),
		},
		{
			name:    "malformed entries are skipped",
			lookup:  func() (*Group, error) { return lookupGroup("bad") },
			wantErr: osuser.UnknownGroupError("bad"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestFilesLookupMissingFile(t *testing.T) {
	oldPasswd := passwdFile
	passwdFile = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { passwdFile = oldPasswd })

	_, err := lookupUnixUid(0)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
