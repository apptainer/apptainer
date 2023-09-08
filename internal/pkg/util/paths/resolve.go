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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
)

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
func Resolve(fileList []string) ([]string, []string, error) {
	machine, err := elfMachine()
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve ELF machine ID: %v", err)
	}
	ldCache, err := ldCache()
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve ld cache: %v", err)
	}

	// Track processed binaries/libraries to eliminate duplicates
	bins := make(map[string]struct{})
	libs := make(map[string]struct{})

	var libraries []string
	var binaries []string

	boundLibsDir := "/.singularity.d/libs"
	boundLibs, err := os.ReadDir(boundLibsDir)
	if err == nil {
		// Inherit all libraries from a parent
		for _, boundLib := range boundLibs {
			libName := boundLib.Name()
			libs[libName] = struct{}{}
			libraries = append(libraries, filepath.Join(boundLibsDir, libName))
		}
	}

	for _, file := range fileList {
		if strings.Contains(file, ".so") {
			// If we have an absolute path, add it 'as-is', plus any symlinks that resolve to it
			if filepath.IsAbs(file) {
				elib, err := elf.Open(file)
				if err != nil {
					sylog.Debugf("ignoring library %s: %s", file, err)
					continue
				}

				if elib.Machine == machine {
					libraries = append(libraries, file)
					links, err := soLinks(file)
					if err != nil {
						sylog.Warningf("ignoring symlinks to %s: %v", file, err)
					} else {
						libraries = append(libraries, links...)
					}
				}
				if err := elib.Close(); err != nil {
					sylog.Warningf("Could not close ELIB: %v", err)
				}
			} else {
				for libName, libPath := range ldCache {
					if !strings.HasPrefix(libName, file) {
						continue
					}
					if _, ok := libs[libName]; !ok {
						elib, err := elf.Open(libPath)
						if err != nil {
							sylog.Debugf("ignoring library %s: %s", libName, err)
							continue
						}

						if elib.Machine == machine {
							libs[libName] = struct{}{}
							libraries = append(libraries, libPath)
						}
						if err := elib.Close(); err != nil {
							sylog.Warningf("Could not close ELIB: %v", err)
						}
					}
				}
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

	return libraries, binaries, nil
}

// ldcache retrieves a map of <library>.so[.version] to its absolute path using
// the system ld cache via `ldconfig -p`. We only take the first instance of
// each <library>.so[.version] from `ldconfig -p` output. I.E. if `ldconfig -p`
// lists three variants of libEGL.so.1 that are in different locations, we only
// report the first, highest priority, variant.
func ldCache() (map[string]string, error) {
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

	// store library name with associated path
	ldCache := make(map[string]string)
	for _, match := range r.FindAllSubmatch(out, -1) {
		if match != nil {
			// libName is the "libnvidia-ml.so.1" (from the above example)
			// libPath is the "/usr/lib64/nvidia/libnvidia-ml.so.1" (from the above example)
			libName := strings.TrimSpace(string(match[1]))
			libPath := strings.TrimSpace(string(match[2]))

			// Only take the first entry for a given <library>.so[.version] in the ldconfig output
			if _, ok := ldCache[libName]; !ok {
				ldCache[libName] = libPath
			}

		}
	}
	return ldCache, nil
}

// elfMachine returns the ELF Machine ID for this system, w.r.t the currently running process
func elfMachine() (machine elf.Machine, err error) {
	// get elf machine to match correct libraries during ldconfig lookup
	self, err := elf.Open("/proc/self/exe")
	if err != nil {
		return 0, fmt.Errorf("could not open /proc/self/exe: %v", err)
	}
	if err := self.Close(); err != nil {
		sylog.Warningf("Could not close ELF: %v", err)
	}
	return self.Machine, nil
}
