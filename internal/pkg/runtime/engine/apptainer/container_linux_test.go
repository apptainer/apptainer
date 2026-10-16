// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"syscall"
	"testing"

	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"gotest.tools/v3/assert"
)

func TestBindPathFlags(t *testing.T) {
	const (
		bind   = uintptr(syscall.MS_BIND | syscall.MS_REC)
		nosuid = uintptr(syscall.MS_NOSUID)
		nodev  = uintptr(syscall.MS_NODEV)
	)
	tests := []struct {
		name        string
		flags       uintptr
		dev, suid   bool
		wantFlags   uintptr
		wantRemount bool
	}{
		{"Default", bind | nosuid | nodev, false, false, bind | nosuid | nodev, true},
		{"Dev", bind | nosuid | nodev, true, false, bind | nosuid, true},
		{"Suid", bind | nosuid | nodev, false, true, bind | nodev, true},
		{"DevSuid", bind | nosuid | nodev, true, true, bind, false},
		{"AllowSetuidDefault", bind | nodev, false, false, bind | nodev, true},
		{"AllowSetuidDev", bind | nodev, true, false, bind, false},
		{"AllowSetuidSuid", bind | nodev, false, true, bind | nodev, true},
		{"AllowSetuidDevSuid", bind | nodev, true, true, bind, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := apptainerconf.BindPathEntry{Source: "/a", Destination: "/a", Dev: tt.dev, Suid: tt.suid}
			flags, remount := bindPathFlags(tt.flags, bp)
			assert.Equal(t, flags, tt.wantFlags)
			assert.Equal(t, remount, tt.wantRemount)
		})
	}
}
