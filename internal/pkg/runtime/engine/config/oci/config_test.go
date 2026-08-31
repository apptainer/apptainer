//go:build linux && seccomp

package oci

import (
	"syscall"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestDefaultConfigSeccompDefaultErrno(t *testing.T) {
	gen, err := DefaultConfigV1()
	if err != nil {
		t.Fatalf("DefaultConfigV1() failed: %v", err)
	}
	if gen.Config.Linux.Seccomp == nil {
		t.Fatal("DefaultConfigV1() returned no seccomp profile")
	}
	if gen.Config.Linux.Seccomp.DefaultErrnoRet != nil {
		t.Fatal("default seccomp errno must remain unspecified")
	}
	for _, call := range gen.Config.Linux.Seccomp.Syscalls {
		if call.Action == specs.ActErrno && call.ErrnoRet != nil &&
			*call.ErrnoRet == uint(syscall.EPERM) {
			t.Fatal("default seccomp profile must not contain explicit EPERM rules")
		}
	}
}
