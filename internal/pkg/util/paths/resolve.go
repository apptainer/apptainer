// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package paths

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

const (
	// ContainerLibsDir is the directory in a container into which host
	// libraries are bound.
	ContainerLibsDir = "/.singularity.d/libs"

	// ContainerCompat32LibsDir is the directory in a container into which
	// 32-bit compatibility host libraries are bound. It is kept separate
	// from ContainerLibsDir because the 32-bit and 64-bit builds of a
	// library have the same name, and both are needed at once.
	ContainerCompat32LibsDir = "/.singularity.d/libs32"
)

// errNoCompat32 is returned when the host architecture has no 32-bit
// counterpart that we know how to provision libraries for.
var errNoCompat32 = errors.New("32-bit compatibility libraries are not supported on this architecture")

// compat32Machine maps the ELF machine of a host to the ELF machine of the
// libraries that 32-bit programs running on that host need. Only x86 is
// listed, as that is where GPU vendors ship 32-bit driver libraries.
var compat32Machine = map[elf.Machine]elf.Machine{
	elf.EM_X86_64: elf.EM_386,
}

// soLinks returns a list of versioned symlinks resolving to a specified library file
func soLinks(libPath string) (paths []string, err error) {
	bareLibPath := strings.SplitAfter(libPath, ".so")[0]
	libCandidates := []string{}
	libGlobPaths, _ := filepath.Glob(fmt.Sprintf("%s*", bareLibPath))
	if len(libGlobPaths) == 0 {
		// should have at least found current lib
		return paths, fmt.Errorf("library not found: %s", libPath)
	}
	// check all files with a similar name (up to .so extension) and
	// work out which are symlinks rather than regular files
	for _, lPath := range libGlobPaths {
		if fi, err := os.Lstat(lPath); err == nil {
			if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
				libCandidates = append(libCandidates, lPath)
			}
		} else {
			sylog.Warningf("error extracting file info for %s: %v", lPath, err)
		}
	}
	// resolve symlinks and check if they eventually point to driver
	for _, lPath := range libCandidates {
		if resolvedLib, err := filepath.EvalSymlinks(lPath); err == nil {
			if resolvedLib == libPath {
				// symlinkCandidate resolves (eventually) to required lib
				sylog.Debugf("Identified %s as a symlink for %s", lPath, libPath)
				paths = append(paths, lPath)
			}
		} else {
			// error resolving symlink?
			sylog.Warningf("unable to resolve symlink for %s: %v", lPath, err)
		}
	}
	return paths, nil
}

// Resolve takes a list of library/binary files (absolute paths, or bare filenames) and processes them into lists of
// resolved library and binary paths to be bound into the container.
func Resolve(fileList []string) ([]string, []string, []string, error) {
	machine, class, err := elfMachine()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not retrieve ELF machine ID: %v", err)
	}
	ldCache, err := libraryCache()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not retrieve library cache: %v", err)
	}

	// Track processed binaries to eliminate duplicates
	bins := make(map[string]struct{})
	filesMap := make(map[string]struct{})

	var binaries []string
	var files []string

	libraries := resolveLibs(fileList, machine, class, ldCache, ContainerLibsDir)

	// A module a loader opens by path, the GBM backend or an X server module,
	// is bound at its host path as well: a container laid out like the host
	// finds it there without being told, and one laid out differently is told
	// through the configuration file bound next.
	for _, lib := range libraries {
		if !isModulePath(lib) || strings.HasPrefix(lib, ContainerLibsDir) {
			continue
		}
		bind := lib + ":" + lib
		if _, ok := filesMap[bind]; !ok {
			filesMap[bind] = struct{}{}
			files = append(files, bind)
		}
	}

	prefixes := filePrefixes()
	for _, file := range fileList {
		if strings.Contains(file, ".so") {
			// libraries are handled by resolveLibs above
			continue
		}
		if filepath.IsAbs(file) {
			src, ok := resolveFile(file, prefixes)
			if !ok {
				continue
			}
			bind := file
			if src != file {
				bind = src + ":" + file
			}
			if _, ok := filesMap[bind]; !ok {
				filesMap[bind] = struct{}{}
				files = append(files, bind)
			}
		} else {
			// treat the file as a binary file - find on PATH and add it to the bind list
			binary, err := exec.LookPath(file)
			if err != nil {
				continue
			}
			if _, ok := bins[binary]; !ok {
				bins[binary] = struct{}{}
				binaries = append(binaries, binary)
			}
		}
	}

	return libraries, binaries, files, nil
}

// ResolveCompat32 takes a list of library/binary files (absolute paths, or
// bare filenames) and resolves the libraries in that list to the 32-bit
// variants present on the host, which are to be bound into the container's
// 32-bit compatibility library directory. Binaries and non-library files in
// the list are ignored, as only libraries have a useful 32-bit counterpart.
func ResolveCompat32(fileList []string) ([]string, error) {
	machine, _, err := elfMachine()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve ELF machine ID: %v", err)
	}
	machine32, ok := compat32Machine[machine]
	if !ok {
		return nil, errNoCompat32
	}
	ldCache, err := libraryCache()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve library cache: %v", err)
	}

	return resolveLibs(fileList, machine32, elf.ELFCLASS32, ldCache, ContainerCompat32LibsDir), nil
}

// resolveLibs resolves the library entries of fileList to absolute paths on
// the host, considering only libraries built for the given ELF machine and
// class. Non-library entries are ignored. Libraries already present in
// boundLibsDir, i.e. inherited from a parent container, are carried over and
// take precedence over anything found on the host.
func resolveLibs(fileList []string, machine elf.Machine, class elf.Class, ldCache map[string][]string, boundLibsDir string) []string {
	// Track processed libraries to eliminate duplicates
	libs := make(map[string]struct{})

	var libraries []string

	if boundLibs, err := os.ReadDir(boundLibsDir); err == nil {
		// Inherit all libraries from a parent
		for _, boundLib := range boundLibs {
			if boundLib.IsDir() {
				continue
			}
			libName := boundLib.Name()
			libs[libName] = struct{}{}
			libraries = append(libraries, filepath.Join(boundLibsDir, libName))
		}
	}

	for _, file := range fileList {
		if !strings.Contains(file, ".so") {
			continue
		}

		// If we have an absolute path, add it 'as-is', plus any symlinks that resolve to it
		if filepath.IsAbs(file) {
			if !matchingLib(file, machine, class) {
				continue
			}
			libraries = append(libraries, file)
			links, err := soLinks(file)
			if err != nil {
				sylog.Warningf("ignoring symlinks to %s: %v", file, err)
				continue
			}
			libraries = append(libraries, links...)
			continue
		}

		for libName, libPaths := range ldCache {
			if !strings.HasPrefix(libName, file) {
				continue
			}
			if _, ok := libs[libName]; ok {
				continue
			}
			// The ld cache may hold several variants of a library, e.g. a
			// 64-bit and a 32-bit build on a multiarch host. Take the
			// first one built for the machine and class we want.
			for _, libPath := range libPaths {
				if !matchingLib(libPath, machine, class) {
					continue
				}
				libs[libName] = struct{}{}
				libraries = append(libraries, libPath)
				break
			}
		}
	}

	return libraries
}

// matchingLib returns true if the file at libPath is an ELF library built for
// the given machine and class.
func matchingLib(libPath string, machine elf.Machine, class elf.Class) bool {
	elib, err := elf.Open(libPath)
	if err != nil {
		sylog.Debugf("ignoring library %s: %s", libPath, err)
		return false
	}
	match := elib.Machine == machine && elib.Class == class
	if err := elib.Close(); err != nil {
		sylog.Warningf("Could not close ELIB: %v", err)
	}
	return match
}

// libraryCache retrieves a map of <library>.so[.version] to the absolute paths
// at which it can be found.
//
// The directories configured as 'gpu library path' in apptainer.conf are
// searched first, in order, and the paths from the system ld cache follow. The
// ld cache is only an optional source: it is skipped, with a warning, when
// ldconfig is not available or fails to run. This allows Apptainer to find the
// GPU driver libraries on systems that do not populate an ld.so cache, or that
// do not ship ldconfig at all, such as NixOS and Guix.
//
// As with ldCache, all of the paths found for a given <library>.so[.version]
// are kept, in search order, so that callers can pick the variant built for
// the architecture they are resolving for.
func libraryCache() (map[string][]string, error) {
	libCache := make(map[string][]string)

	searchPaths := gpuLibraryPath()
	if len(searchPaths) > 0 {
		sylog.Debugf("Searching for GPU libraries in configured 'gpu library path': %s", strings.Join(searchPaths, ", "))
	}
	for _, dir := range searchPaths {
		addDirToCache(libCache, dir)
	}

	ldCache, err := ldCache()
	if err != nil {
		// ldconfig is not essential when the libraries can be found via
		// the configured search paths.
		if len(libCache) == 0 {
			sylog.Warningf("Could not read the ld cache, and no 'gpu library path' is configured in apptainer.conf: %v", err)
		} else {
			sylog.Debugf("Not using the ld cache: %v", err)
		}
	}
	for libName, libPaths := range ldCache {
		for _, libPath := range libPaths {
			if !slices.Contains(libCache[libName], libPath) {
				libCache[libName] = append(libCache[libName], libPath)
			}
		}
	}
	// The module directories next to the library directories, those of
	// the configured search paths first.
	var cacheDirs []string
	for _, libPaths := range ldCache {
		for _, libPath := range libPaths {
			cacheDirs = append(cacheDirs, filepath.Dir(libPath))
		}
	}
	slices.Sort(cacheDirs)
	for _, dir := range moduleDirs(append(searchPaths, slices.Compact(cacheDirs)...)) {
		addDirToCache(libCache, dir)
	}

	return libCache, nil
}

// moduleSubdirs are the directories, relative to a library directory or its
// parent, in which a driver installs the modules that are loaded by path
// rather than through the dynamic loader, so the ld cache never lists them:
// the GBM backend and the X server's driver and GLX modules.
var moduleSubdirs = []string{
	"gbm", "nvidia/xorg",
	"xorg/modules/drivers", "xorg/modules/extensions",
	"xorg/modules/updates/drivers", "xorg/modules/updates/extensions",
}

// moduleDirs returns the module directories found next to the given library
// directories and next to their parents, which is where Debian keeps
// /usr/lib/xorg/modules relative to /usr/lib/<multiarch>, in that order.
func moduleDirs(libDirs []string) []string {
	seen := make(map[string]struct{})
	var dirs []string
	for _, libDir := range libDirs {
		for _, base := range []string{libDir, filepath.Dir(libDir)} {
			for _, sub := range moduleSubdirs {
				dir := filepath.Join(base, sub)
				if _, ok := seen[dir]; ok {
					continue
				}
				seen[dir] = struct{}{}
				if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
					dirs = append(dirs, dir)
				}
			}
		}
	}
	return dirs
}

// isModulePath reports whether libPath sits in one of the module directories,
// where a loader opens it by path rather than through the dynamic loader.
func isModulePath(libPath string) bool {
	dir := filepath.Dir(libPath)
	for _, sub := range moduleSubdirs {
		if strings.HasSuffix(dir, "/"+sub) {
			return true
		}
	}
	return false
}

// alternateDirs lists, for a directory a container's loaders read, the
// directories a distribution may install the same files in instead.
var alternateDirs = map[string][]string{
	"/usr/share/X11/xorg.conf.d":    {"/etc/X11/xorg.conf.d"},
	"/usr/share/glvnd/egl_vendor.d": {"/etc/glvnd/egl_vendor.d"},
}

// filePrefixes returns the installation prefixes above the configured 'gpu
// library path' directories, under which a driver installed outside the
// standard tree keeps its share and etc directories beside its libraries.
func filePrefixes() []string {
	var prefixes []string
	for _, dir := range gpuLibraryPath() {
		prefix := filepath.Dir(filepath.Clean(dir))
		if prefix != "/" && !slices.Contains(prefixes, prefix) {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

// resolveFile finds the host file for a configuration entry, which names the
// path a container's loaders read: the same path on the host first, then the
// alternate directories, then the file under each prefix (its /usr part
// dropped, so that /usr/share/x becomes <prefix>/share/x and /etc/x becomes
// <prefix>/etc/x). The entry stays the destination in the container.
func resolveFile(entry string, prefixes []string) (string, bool) {
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
	if exists(entry) {
		return entry, true
	}
	dir, name := filepath.Split(entry)
	for _, alt := range alternateDirs[filepath.Clean(dir)] {
		if candidate := filepath.Join(alt, name); exists(candidate) {
			return candidate, true
		}
	}
	for _, prefix := range prefixes {
		if candidate := filepath.Join(prefix, strings.TrimPrefix(entry, "/usr")); exists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// gpuLibraryPath returns the directories configured as 'gpu library path' in
// apptainer.conf, to be searched for GPU driver libraries.
func gpuLibraryPath() []string {
	config := apptainerconf.GetCurrentConfig()
	if config == nil {
		// The configuration has not been loaded, e.g. in a unit test.
		return nil
	}

	dirs := make([]string, 0, len(config.GpuLibraryPath))
	for _, dir := range config.GpuLibraryPath {
		dir = strings.TrimSpace(dir)
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// addDirToCache adds the libraries directly contained in dir to libCache,
// after the paths already present from a higher priority source.
func addDirToCache(libCache map[string][]string, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		sylog.Debugf("Skipping GPU library path %s: %v", dir, err)
		return
	}
	for _, entry := range entries {
		libName := entry.Name()
		if !strings.Contains(libName, ".so") {
			continue
		}
		libPath := filepath.Join(dir, libName)
		// Versioned libraries are usually symlinks, so resolve the entry
		// rather than requiring a regular file here. Skipping entries
		// that do not resolve, e.g. dangling symlinks, keeps them from
		// being offered ahead of a working library of the same name
		// found later.
		fi, err := os.Stat(libPath)
		if err != nil {
			sylog.Debugf("Skipping GPU library %s: %v", libPath, err)
			continue
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		if !slices.Contains(libCache[libName], libPath) {
			libCache[libName] = append(libCache[libName], libPath)
		}
	}
}

// ldCache retrieves a map of <library>.so[.version] to the absolute paths at
// which it can be found, using the system ld cache via `ldconfig -p`. All of
// the paths listed for a given <library>.so[.version] are kept, in `ldconfig
// -p` priority order, so that callers can pick the variant built for the
// architecture they are resolving for.
func ldCache() (map[string][]string, error) {
	// walk through the ldconfig output and add entries which contain the filenames
	// returned by nvidia-container-cli OR the nvliblist.conf file contents
	ldconfig, err := bin.FindBin("ldconfig")
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(ldconfig, "-p").Output()
	if err != nil {
		return nil, fmt.Errorf("could not execute ldconfig: %v", err)
	}

	// sample ldconfig -p output:
	// libnvidia-ml.so.1 (libc6,x86-64) => /usr/lib64/nvidia/libnvidia-ml.so.1
	r, err := regexp.Compile(`(?m)^(.*)\s*\(.*\)\s*=>\s*(.*)$`)
	if err != nil {
		return nil, fmt.Errorf("could not compile ldconfig regexp: %v", err)
	}

	// store library name with associated paths
	ldCache := make(map[string][]string)
	for _, match := range r.FindAllSubmatch(out, -1) {
		if match != nil {
			// libName is the "libnvidia-ml.so.1" (from the above example)
			// libPath is the "/usr/lib64/nvidia/libnvidia-ml.so.1" (from the above example)
			libName := strings.TrimSpace(string(match[1]))
			libPath := strings.TrimSpace(string(match[2]))

			if !slices.Contains(ldCache[libName], libPath) {
				ldCache[libName] = append(ldCache[libName], libPath)
			}
		}
	}
	return ldCache, nil
}

// elfMachine returns the ELF Machine ID and class for this system, w.r.t the
// currently running process
func elfMachine() (machine elf.Machine, class elf.Class, err error) {
	// get elf machine to match correct libraries during ldconfig lookup
	self, err := elf.Open("/proc/self/exe")
	if err != nil {
		return 0, 0, fmt.Errorf("could not open /proc/self/exe: %v", err)
	}
	machine, class = self.Machine, self.Class
	if err := self.Close(); err != nil {
		sylog.Warningf("Could not close ELF: %v", err)
	}
	return machine, class, nil
}
