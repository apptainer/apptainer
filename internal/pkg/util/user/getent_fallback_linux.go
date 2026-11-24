// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

// Static builds cannot use the host's NSS modules, so users and groups
// that come only from sources such as LDAP or SSSD are not found by the
// in-process lookups. In those builds, ask the host's own getent, which
// is dynamically linked and uses the host's NSS configuration.

//go:build getent_fallback && linux

package user

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
)

// findGetent returns the path of getent, found with bin.FindBin. It is
// not looked up before the configuration and binary path are loaded, as
// for the lookup of the current user when the command line is set up.
var findGetent = func() (string, error) {
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil || cfg.SuidBinaryPath == "" {
		return "", fmt.Errorf("binary path is not set yet")
	}
	return bin.FindBin("getent")
}

// getentTimeout bounds a getent call, which may need the network.
const getentTimeout = 10 * time.Second

// getentAllowed reports whether it is safe to run getent. It is not run
// with host root privileges or with differing real and effective IDs,
// as in a setuid program.
var getentAllowed = func() bool {
	return namespaces.IsUnprivileged() &&
		os.Getuid() == os.Geteuid() && os.Getgid() == os.Getegid()
}

// getent returns the fields of the single database entry for key, or nil
// if getent is unavailable or the entry is not found.
func getent(database, key string, nfields int) []string {
	if !getentAllowed() {
		return nil
	}
	path, err := findGetent()
	if err != nil {
		sylog.Debugf("Not looking up %s %s with getent: %v", database, key, err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), getentTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, database, key)
	// An empty environment, so that nothing such as LD_PRELOAD is
	// passed to the dynamically linked getent
	cmd.Env = []string{}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// getent exits with 2 when the key is not found
		sylog.Debugf("%s %s %s: %v", path, database, key, err)
		return nil
	}
	line := strings.TrimSpace(stdout.String())
	if strings.Contains(line, "\n") {
		return nil
	}
	fields := strings.Split(line, ":")
	if len(fields) != nfields {
		return nil
	}
	sylog.Debugf("Found %s entry for %s with %s", database, key, path)
	return fields
}

// matches reports whether an entry with the given name and numeric ID
// is the one that key asked for. getent treats a numeric key as an ID,
// so only the field that was asked for is compared: for a name lookup of
// "0", the root entry that getent returns does not match.
func matches(key string, byID bool, name, id string) bool {
	if byID {
		return key == id
	}
	return key == name
}

func getentUserFallback(key string, byID bool, u *User, err error) (*User, error) {
	f := getent("passwd", key, 7)
	if f == nil || !matches(key, byID, f[0], f[2]) {
		return u, err
	}
	uid, uerr := strconv.ParseUint(f[2], 10, 32)
	gid, gerr := strconv.ParseUint(f[3], 10, 32)
	if uerr != nil || gerr != nil {
		return u, err
	}
	return &User{
		Name:  f[0],
		UID:   uint32(uid),
		GID:   uint32(gid),
		Gecos: f[4],
		Dir:   f[5],
		Shell: f[6],
	}, nil
}

func getentGroupFallback(key string, byID bool, g *Group, err error) (*Group, error) {
	f := getent("group", key, 4)
	if f == nil || !matches(key, byID, f[0], f[2]) {
		return g, err
	}
	gid, gerr := strconv.ParseUint(f[2], 10, 32)
	if gerr != nil {
		return g, err
	}
	return &Group{Name: f[0], GID: uint32(gid)}, nil
}
