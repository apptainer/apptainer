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
	"slices"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
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

// setGpuLibraryPath sets the 'gpu library path' configuration directive for
// the duration of a test.
func setGpuLibraryPath(t *testing.T, dirs ...string) {
	t.Helper()

	old := apptainerconf.GetCurrentConfig()
	t.Cleanup(func() { apptainerconf.SetCurrentConfig(old) })

	config := &apptainerconf.File{}
	if old != nil {
		copied := *old
		config = &copied
	}
	config.GpuLibraryPath = dirs
	apptainerconf.SetCurrentConfig(config)
}

// TestLibraryCacheSearchPaths checks that the directories configured as 'gpu
// library path' are searched, in order, ahead of the ld cache.
func TestLibraryCacheSearchPaths(t *testing.T) {
	firstDir := filepath.Join(t.TempDir(), "first")
	secondDir := filepath.Join(t.TempDir(), "second")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Could not create dir: %v", err)
		}
	}

	// libboth.so.1 is in both directories, and the one in the directory
	// configured first must be listed first. libsecond.so.1 is only in the
	// second.
	firstBoth := filepath.Join(firstDir, "libboth.so.1")
	secondBoth := filepath.Join(secondDir, "libboth.so.1")
	for _, lib := range []string{firstBoth, secondBoth, filepath.Join(secondDir, "libsecond.so.1")} {
		if err := os.WriteFile(lib, nil, 0o644); err != nil {
			t.Fatalf("Could not create file: %v", err)
		}
	}
	// A non-library file must be ignored.
	if err := os.WriteFile(filepath.Join(firstDir, "notalib.txt"), nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	// A dangling symlink must not be listed ahead of the working library of
	// the same name in the second directory.
	shadowed := filepath.Join(secondDir, "libshadowed.so.1")
	if err := os.WriteFile(shadowed, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	if err := os.Symlink(filepath.Join(firstDir, "nonexistent.so"), filepath.Join(firstDir, "libshadowed.so.1")); err != nil {
		t.Fatalf("Could not symlink: %v", err)
	}

	setGpuLibraryPath(t, firstDir, secondDir, filepath.Join(firstDir, "nonexistent"))

	gotCache, err := libraryCache()
	if err != nil {
		t.Fatalf("libraryCache() error = %v", err)
	}
	if want := []string{firstBoth, secondBoth}; !reflect.DeepEqual(gotCache["libboth.so.1"], want) {
		t.Errorf("libraryCache() gave libboth.so.1 = %q, expected %q", gotCache["libboth.so.1"], want)
	}
	if _, ok := gotCache["libsecond.so.1"]; !ok {
		t.Error("libraryCache() did not include libsecond.so.1 from the second search path")
	}
	if _, ok := gotCache["notalib.txt"]; ok {
		t.Error("libraryCache() included a non-library file")
	}
	if want := []string{shadowed}; !reflect.DeepEqual(gotCache["libshadowed.so.1"], want) {
		t.Errorf("libraryCache() gave libshadowed.so.1 = %q, expected the dangling symlink to be skipped in favor of %q", gotCache["libshadowed.so.1"], want)
	}
}

// TestLibraryCacheModuleDirs checks that the module directories next to a
// library directory, and next to its parent, are searched for the modules the
// ld cache never lists.
func TestLibraryCacheModuleDirs(t *testing.T) {
	libDir := filepath.Join(t.TempDir(), "lib", "x86_64-linux-gnu")
	if err := os.MkdirAll(filepath.Join(libDir, "gbm"), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}
	allocator := filepath.Join(libDir, "libtestgpu-allocator.so.1")
	if err := os.WriteFile(allocator, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	// The driver installs its GBM backend as a symlink to the allocator
	// library.
	backend := filepath.Join(libDir, "gbm", "testgpu-drm_gbm.so")
	if err := os.Symlink("../libtestgpu-allocator.so.1", backend); err != nil {
		t.Fatalf("Could not symlink: %v", err)
	}
	modules := filepath.Join(filepath.Dir(libDir), "xorg", "modules")
	driver := filepath.Join(modules, "drivers", "testgpu_drv.so")
	glx := filepath.Join(modules, "extensions", "libglxserver_testgpu.so.1")
	for _, module := range []string{driver, glx} {
		if err := os.MkdirAll(filepath.Dir(module), 0o755); err != nil {
			t.Fatalf("Could not create dir: %v", err)
		}
		if err := os.WriteFile(module, nil, 0o644); err != nil {
			t.Fatalf("Could not create file: %v", err)
		}
	}

	setGpuLibraryPath(t, libDir)

	gotCache, err := libraryCache()
	if err != nil {
		t.Fatalf("libraryCache() error = %v", err)
	}
	for name, want := range map[string]string{
		"libtestgpu-allocator.so.1": allocator,
		"testgpu-drm_gbm.so":        backend,
		"testgpu_drv.so":            driver,
		"libglxserver_testgpu.so.1": glx,
	} {
		if got := gotCache[name]; !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("libraryCache() gave %s = %q, expected %q", name, got, want)
		}
	}
}

// TestLibraryCacheWithoutLdconfig checks that a configured 'gpu library path'
// is sufficient on its own, i.e. that resolution does not fail when the ld
// cache is unavailable. bin.FindBin("ldconfig") fails in the unit test
// environment because the installed apptainer.conf is not present, which is
// the same code path taken on a host without ldconfig.
func TestLibraryCacheWithoutLdconfig(t *testing.T) {
	if _, err := ldCache(); err == nil {
		t.Skip("ldconfig is available in this environment")
	}

	dir := t.TempDir()
	lib := filepath.Join(dir, "libtest.so.1")
	if err := os.WriteFile(lib, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	setGpuLibraryPath(t, dir)

	gotCache, err := libraryCache()
	if err != nil {
		t.Fatalf("libraryCache() error = %v, expected the missing ld cache to be tolerated", err)
	}
	if want := []string{lib}; !reflect.DeepEqual(gotCache["libtest.so.1"], want) {
		t.Errorf("libraryCache() gave libtest.so.1 = %q, expected %q", gotCache["libtest.so.1"], want)
	}
}

// TestIsModulePath checks that only the module directories, where a loader
// opens a file by path, count as module paths.
func TestIsModulePath(t *testing.T) {
	for path, want := range map[string]bool{
		"/usr/lib/x86_64-linux-gnu/gbm/nvidia-drm_gbm.so":           true,
		"/usr/lib/xorg/modules/drivers/nvidia_drv.so":               true,
		"/usr/lib/xorg/modules/extensions/libglxserver_nvidia.so.1": true,
		"/usr/lib/x86_64-linux-gnu/nvidia/xorg/nvidia_drv.so":       true,
		"/usr/lib/x86_64-linux-gnu/libcuda.so.1":                    false,
		"/usr/lib/x86_64-linux-gnu/libnvidia-allocator.so.1":        false,
		"/.singularity.d/libs/nvidia-drm_gbm.so":                    false,
	} {
		if got := isModulePath(path); got != want {
			t.Errorf("isModulePath(%q) = %v, expected %v", path, got, want)
		}
	}
}

// TestResolveFile checks that a configuration file is found at its own path,
// else under the prefix above a 'gpu library path' directory, with the entry
// kept as the destination.
func TestResolveFile(t *testing.T) {
	prefix := t.TempDir()
	found := filepath.Join(prefix, "share", "glvnd", "egl_vendor.d", "10_testgpu.json")
	if err := os.MkdirAll(filepath.Dir(found), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}
	if err := os.WriteFile(found, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	etcFound := filepath.Join(prefix, "etc", "OpenCL", "vendors", "testgpu.icd")
	if err := os.MkdirAll(filepath.Dir(etcFound), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}
	if err := os.WriteFile(etcFound, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	prefixes := []string{prefix}

	for entry, want := range map[string]string{
		"/usr/share/glvnd/egl_vendor.d/10_testgpu.json": found,
		"/etc/OpenCL/vendors/testgpu.icd":               etcFound,
		found:                                           found,
	} {
		got, ok := resolveFile(entry, prefixes)
		if !ok || got != want {
			t.Errorf("resolveFile(%q) = %q, %v, expected %q", entry, got, ok, want)
		}
	}
	if got, ok := resolveFile("/usr/share/glvnd/egl_vendor.d/absent.json", prefixes); ok {
		t.Errorf("resolveFile() found an absent file at %q", got)
	}
}

// TestFilePrefixes checks that the prefixes are the parents of the configured
// directories, without duplicates and without the root.
func TestFilePrefixes(t *testing.T) {
	setGpuLibraryPath(t, "/run/opengl-driver/lib", "/run/opengl-driver/lib32", "/lib")
	got := filePrefixes()
	if want := []string{"/run/opengl-driver"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filePrefixes() = %q, expected %q", got, want)
	}
}

// TestResolveModulesAtHostPaths checks that a module found through a
// configured directory is bound at its host path as a file, next to its
// place among the libraries, and that a configuration file found under the
// prefix is bound at the standard path.
func TestResolveModulesAtHostPaths(t *testing.T) {
	machine, class, err := elfMachine()
	if err != nil {
		t.Fatalf("elfMachine() error = %v", err)
	}
	libDir := filepath.Join(t.TempDir(), "lib")
	if err := os.MkdirAll(filepath.Join(libDir, "gbm"), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}
	// An ELF library of the host's own machine and class, copied from the
	// one every test host has.
	lib, err := os.ReadFile(hostLibc(t))
	if err != nil {
		t.Fatalf("Could not read a host library: %v", err)
	}
	backend := filepath.Join(libDir, "gbm", "testgpu-drm_gbm.so")
	if err := os.WriteFile(backend, lib, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	if !matchingLib(backend, machine, class) {
		t.Skip("the copied host library does not match this machine")
	}
	config := filepath.Join(filepath.Dir(libDir), "share", "egl", "egl_external_platform.d", "15_testgpu_gbm.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatalf("Could not create dir: %v", err)
	}
	if err := os.WriteFile(config, nil, 0o644); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}
	setGpuLibraryPath(t, libDir)

	libs, _, files, err := Resolve([]string{"testgpu-drm_gbm.so", "/usr/share/egl/egl_external_platform.d/15_testgpu_gbm.json"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !slices.Contains(libs, backend) {
		t.Errorf("Resolve() libraries %q lack the backend %q", libs, backend)
	}
	for _, want := range []string{
		backend + ":" + backend,
		config + ":/usr/share/egl/egl_external_platform.d/15_testgpu_gbm.json",
	} {
		if !slices.Contains(files, want) {
			t.Errorf("Resolve() files %q lack %q", files, want)
		}
	}
}

// hostLibc returns the path of the C library the test binary itself links,
// an ELF library of the host's machine and class.
func hostLibc(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{
		"/lib/x86_64-linux-gnu/libc.so.6", "/lib64/libc.so.6", "/lib/aarch64-linux-gnu/libc.so.6",
		"/usr/lib/libc.so.6", "/lib/libc.so.6",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skip("no C library found to copy")
	return ""
}
