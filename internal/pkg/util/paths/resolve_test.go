// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build linux

package paths

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var testLibList = []string{"libc.so", "echo"}

func TestElfMachine(t *testing.T) {
	gotMachine, gotClass, err := elfMachine()
	if err != nil {
		t.Errorf("elfMachine() error = %v", err)
		return
	}
	if gotMachine <= 0 {
		t.Errorf("elfMachine() gotMachine = %v is <=0", gotMachine)
	}
	if gotClass != elf.ELFCLASS32 && gotClass != elf.ELFCLASS64 {
		t.Errorf("elfMachine() gotClass = %v is not a valid ELF class", gotClass)
	}
}

func TestLdCache(t *testing.T) {
	gotCache, err := ldCache()
	if err != nil {
		t.Errorf("ldCache() error = %v", err)
		return
	}
	if len(gotCache) == 0 {
		t.Error("ldCache() gave no results")
	}
	for name, paths := range gotCache {
		if !strings.HasPrefix(name, "ld-linux") {
			continue
		}
		for _, path := range paths {
			if strings.Contains(path, "ld-linux") {
				return
			}
		}
	}
	t.Error("ldCache() result did not include expected ld-linux entry")
}

func TestSoLinks(t *testing.T) {
	// Test link structure:
	// a.so.1.2 -> a.so.1 -> a.so (file)
	//   - soLinks(a.so) should give both of these symlinks
	// a.so.2 -> b.so
	//   - this should *not* get included, as it doesn't resolve back to a.so
	tmpDir := t.TempDir()
	aFile := filepath.Join(tmpDir, "a.so")
	a1Link := filepath.Join(tmpDir, "a.so.1")
	a12Link := filepath.Join(tmpDir, "a.so.1.2")
	if err := os.WriteFile(aFile, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	if err := os.Symlink(aFile, a1Link); err != nil {
		t.Fatalf("Could not symlink: %v", err)
	}
	if err := os.Symlink(aFile, a12Link); err != nil {
		t.Fatalf("Could not symlink: %v", err)
	}
	bFile := filepath.Join(tmpDir, "b.so")
	err := os.WriteFile(bFile, nil, 0o644)
	if err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	a2Link := filepath.Join(tmpDir, "a.so.2")
	if err := os.Symlink(bFile, a2Link); err != nil {
		t.Fatalf("Could not symlink: %v", err)
	}

	expectedLinks := []string{a1Link, a12Link}

	gotLinks, err := soLinks(aFile)
	if err != nil {
		t.Errorf("soLinks() error = %v", err)
		return
	}
	if len(gotLinks) == 0 {
		t.Error("soLinks() gave no results")
	}
	if !reflect.DeepEqual(gotLinks, expectedLinks) {
		t.Errorf("soList() gave unexpected results, got: %v expected: %v", gotLinks, expectedLinks)
	}
}

func TestResolve(t *testing.T) {
	// Test whether the `Resolve` method can return lib, bin and file without errors.
	tmpdir := t.TempDir()
	filePath := filepath.Join(tmpdir, "/usr/share/glvnd/egl_vendor.d/10_nvidia.json")
	err := os.MkdirAll(filepath.Dir(filePath), 0o755)
	if err != nil {
		t.Errorf("unable to create dirs: %v", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		t.Errorf("unable to create file: %v", err)
	}
	f.Close()

	testLibList = append(testLibList, filePath)
	gotLibs, gotBin, gotFiles, err := Resolve(testLibList)
	if err != nil {
		t.Errorf("paths() error = %v", err)
	}
	if len(gotLibs) == 0 {
		t.Error("paths() gave no libraries")
	}
	if len(gotBin) == 0 {
		t.Error("paths() gave no binaries")
	}
	if len(gotFiles) == 0 {
		t.Error("paths() gave no files")
	}
}

// writeELF writes a minimal, headers-only ELF file for the given machine and
// class, so that resolution can be tested without depending on which
// architectures the host happens to have libraries installed for.
func writeELF(t *testing.T, path string, machine elf.Machine, class elf.Class) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}

	ident := [16]byte{0x7f, 'E', 'L', 'F', byte(class), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT)}

	b := new(bytes.Buffer)
	b.Write(ident[:])
	write := func(vals ...any) {
		for _, v := range vals {
			if err := binary.Write(b, binary.LittleEndian, v); err != nil {
				t.Fatalf("Could not write ELF header: %v", err)
			}
		}
	}
	// e_type, e_machine, e_version
	write(uint16(elf.ET_DYN), uint16(machine), uint32(elf.EV_CURRENT))
	if class == elf.ELFCLASS64 {
		// e_entry, e_phoff, e_shoff
		write(uint64(0), uint64(0), uint64(0))
		// e_flags, e_ehsize, e_phentsize, e_phnum, e_shentsize, e_shnum, e_shstrndx
		write(uint32(0), uint16(64), uint16(56), uint16(0), uint16(64), uint16(0), uint16(0))
	} else {
		write(uint32(0), uint32(0), uint32(0))
		write(uint32(0), uint16(52), uint16(32), uint16(0), uint16(40), uint16(0), uint16(0))
	}

	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatalf("Could not write file: %v", err)
	}
}

// TestResolveLibsMultiarch checks that, when the ld cache lists both a 64-bit
// and a 32-bit build of the same library, the one matching the requested
// machine and class is selected, whichever order they are listed in.
func TestResolveLibsMultiarch(t *testing.T) {
	tmpDir := t.TempDir()
	lib64 := filepath.Join(tmpDir, "lib64", "libtest.so.1")
	lib32 := filepath.Join(tmpDir, "lib32", "libtest.so.1")
	writeELF(t, lib64, elf.EM_X86_64, elf.ELFCLASS64)
	writeELF(t, lib32, elf.EM_386, elf.ELFCLASS32)

	// The 32-bit variant is listed first, as ldconfig -p gives no guarantee
	// about the order in which the variants of a library appear.
	ldCache := map[string][]string{"libtest.so.1": {lib32, lib64}}

	tests := []struct {
		name    string
		machine elf.Machine
		class   elf.Class
		want    []string
	}{
		{"64bit", elf.EM_X86_64, elf.ELFCLASS64, []string{lib64}},
		{"32bit", elf.EM_386, elf.ELFCLASS32, []string{lib32}},
		{"unavailable", elf.EM_AARCH64, elf.ELFCLASS64, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveLibs([]string{"libtest.so"}, tt.machine, tt.class, ldCache, filepath.Join(tmpDir, "nonexistent"))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveLibs() got: %v expected: %v", got, tt.want)
			}
		})
	}
}

// TestResolveLibsInheritsBoundLibs checks that libraries bound in by a parent
// container are inherited, and that a subdirectory is not mistaken for one of
// them.
func TestResolveLibsInheritsBoundLibs(t *testing.T) {
	tmpDir := t.TempDir()
	boundLibsDir := filepath.Join(tmpDir, "libs")
	boundLib := filepath.Join(boundLibsDir, "libbound.so.1")
	writeELF(t, boundLib, elf.EM_X86_64, elf.ELFCLASS64)
	if err := os.Mkdir(filepath.Join(boundLibsDir, "subdir"), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}

	got := resolveLibs(nil, elf.EM_X86_64, elf.ELFCLASS64, nil, boundLibsDir)
	want := []string{boundLib}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveLibs() got: %v expected: %v", got, want)
	}
}

func TestResolveCompat32(t *testing.T) {
	machine, _, err := elfMachine()
	if err != nil {
		t.Fatalf("elfMachine() error = %v", err)
	}
	gotLibs, err := ResolveCompat32(testLibList)
	if _, ok := compat32Machine[machine]; !ok {
		if !errors.Is(err, errNoCompat32) {
			t.Errorf("ResolveCompat32() error = %v, expected %v", err, errNoCompat32)
		}
		return
	}
	if err != nil {
		if _, ldErr := ldCache(); ldErr != nil {
			t.Skipf("ld cache is not available in this environment: %v", ldErr)
		}
		t.Errorf("ResolveCompat32() error = %v", err)
		return
	}
	// A host is not required to have 32-bit libraries installed, so an empty
	// result is valid. Anything returned must be a 32-bit library, though.
	for _, lib := range gotLibs {
		if !matchingLib(lib, compat32Machine[machine], elf.ELFCLASS32) {
			t.Errorf("ResolveCompat32() returned %s, which is not a 32-bit library", lib)
		}
	}
}
