// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package capabilities

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
)

func TestGetProcess(t *testing.T) {
	test.EnsurePrivilege(t)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tests := []struct {
		name string
		fn   func() (uint64, error)
		cap  string
	}{
		{
			name: "effective",
			fn:   GetProcessEffective,
			cap:  "CAP_SYS_ADMIN",
		},
		{
			name: "permitted",
			fn:   GetProcessPermitted,
		},
		{
			name: "inheritable",
			fn:   GetProcessInheritable,
		},
	}

	for _, tt := range tests {
		caps, err := tt.fn()
		if err != nil {
			t.Fatalf("unexpected error while getting process %s capabilities: %s", tt.name, err)
		}
		capability := Map[tt.cap]
		if tt.cap != "" && caps&uint64(1<<capability.Value) == 0 {
			t.Fatalf("%s capability %s missing", tt.name, tt.cap)
		}
	}
}

func TestSetProcessEffective(t *testing.T) {
	test.EnsurePrivilege(t)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	data, err := getProcessCapabilities()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		cap          uint64
		oldEffective uint64
	}{
		{
			name:         "set cap_sys_admin only",
			cap:          uint64(1 << Map["CAP_SYS_ADMIN"].Value),
			oldEffective: uint64(data[0].Effective) | uint64(data[1].Effective)<<32,
		},
		{
			name:         "restore capabilities",
			cap:          uint64(data[0].Effective) | uint64(data[1].Effective)<<32,
			oldEffective: uint64(1 << Map["CAP_SYS_ADMIN"].Value),
		},
	}

	for _, tt := range tests {
		old, err := SetProcessEffective(tt.cap)
		if err != nil {
			t.Fatalf("unexpected error for %s: %s", tt.name, err)
		} else if old != tt.oldEffective {
			t.Fatalf("unexpected old effective set for %s", tt.name)
		}
	}
}

// statusCapSet returns the named capability set of the calling thread, as
// /proc reports it, so that a getter can be compared with the kernel's own
// view of the same set.
func statusCapSet(t *testing.T, field string) uint64 {
	t.Helper()

	status, err := os.ReadFile("/proc/thread-self/status")
	if err != nil {
		t.Fatalf("unexpected error while reading the thread status: %s", err)
	}
	prefix := field + ":"
	for _, line := range strings.Split(string(status), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		caps, err := strconv.ParseUint(value, 16, 64)
		if err != nil {
			t.Fatalf("unexpected %s set %q in the thread status: %s", field, value, err)
		}
		return caps
	}
	t.Fatalf("no %s line in the thread status", field)
	return 0
}

func TestGetProcessBounding(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	expected := statusCapSet(t, "CapBnd")

	caps, err := GetProcessBounding()
	if err != nil {
		t.Fatalf("unexpected error while getting process bounding capabilities: %s", err)
	}
	if caps != expected {
		t.Fatalf("bounding set 0x%x differs from 0x%x in the thread status", caps, expected)
	}
}

func TestGetProcessAmbient(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Only the capabilities this kernel knows about can be reported, so the
	// comparison is made over the same map the getter walks.
	var known uint64
	for _, c := range Map {
		known |= uint64(1) << c.Value
	}
	expected := statusCapSet(t, "CapPrm") & statusCapSet(t, "CapBnd") & known

	caps, err := GetProcessAmbient()
	if err != nil {
		t.Fatalf("unexpected error while getting process ambient capabilities: %s", err)
	}
	var got uint64
	for _, c := range caps {
		got |= uint64(1) << c
	}
	if got != expected {
		t.Fatalf("ambient set 0x%x differs from the permitted and bounding sets 0x%x in the thread status", got, expected)
	}
}
