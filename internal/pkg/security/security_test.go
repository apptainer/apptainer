// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package security

import (
	"runtime"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/security/apparmor"
	"github.com/apptainer/apptainer/internal/pkg/security/selinux"
	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestGetParam(t *testing.T) {
	paramTests := []struct {
		security []string
		feature  string
		result   string
	}{
		{
			security: []string{"seccomp:test"},
			feature:  "seccomp",
			result:   "test",
		},
		{
			security: []string{"test:test"},
			feature:  "seccomp",
			result:   "",
		},
		{
			security: []string{"seccomp:test", "uid:1000"},
			feature:  "uid",
			result:   "1000",
		},
	}
	for _, p := range paramTests {
		r := GetParam(p.security, p.feature)
		if p.result != r {
			t.Errorf("unexpected result for param %v, returned %s instead of %s", p.security, r, p.result)
		}
	}
}

func TestConfigure(t *testing.T) {
	test.EnsurePrivilege(t)

	specs := []struct {
		desc          string
		spec          specs.Spec
		expectFailure bool
		disabled      bool
	}{
		{
			desc: "empty security spec",
			spec: specs.Spec{},
		},
		{
			desc: "both SELinux context and apparmor profile",
			spec: specs.Spec{
				Process: &specs.Process{
					SelinuxLabel:    "test",
					ApparmorProfile: "test",
				},
			},
			expectFailure: true,
		},
		{
			desc: "with bad SELinux context",
			spec: specs.Spec{
				Process: &specs.Process{
					SelinuxLabel: "test",
				},
			},
			expectFailure: true,
			disabled:      !selinux.Enabled(),
		},
		{
			desc: "with unconfined SELinux context",
			spec: specs.Spec{
				Process: &specs.Process{
					SelinuxLabel: "unconfined_u:unconfined_r:unconfined_t:s0",
				},
			},
			disabled: !selinux.Enabled(),
		},
		{
			desc: "SELinux when not available",
			spec: specs.Spec{
				Process: &specs.Process{
					SelinuxLabel: "unconfined_u:unconfined_r:unconfined_t:s0",
				},
			},
			expectFailure: true,
			disabled:      selinux.Enabled(),
		},
		{
			desc: "with bad apparmor profile",
			spec: specs.Spec{
				Process: &specs.Process{
					ApparmorProfile: "__test__",
				},
			},
			expectFailure: true,
			disabled:      !apparmor.Enabled(),
		},
		{
			desc: "with unconfined apparmor profile",
			spec: specs.Spec{
				Process: &specs.Process{
					ApparmorProfile: "unconfined",
				},
			},
			disabled: !apparmor.Enabled(),
		},
		{
			desc: "apparmor when not available",
			spec: specs.Spec{
				Process: &specs.Process{
					ApparmorProfile: "unconfined",
				},
			},
			expectFailure: true,
			disabled:      apparmor.Enabled(),
		},
	}

	for _, s := range specs {
		t.Run(s.desc, func(t *testing.T) {
			if s.disabled {
				t.Skip("test disabled, security module not enabled on this system")
			}

			var err error

			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			err = Configure(&s.spec)

			if err != nil && !s.expectFailure {
				t.Errorf("unexpected failure %s: %s", s.desc, err)
			} else if err == nil && s.expectFailure {
				t.Errorf("unexpected success %s", s.desc)
			}
		})
	}
}
