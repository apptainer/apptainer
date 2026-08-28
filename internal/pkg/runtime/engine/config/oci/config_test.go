//go:build linux && seccomp

package oci

import "testing"

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
}
