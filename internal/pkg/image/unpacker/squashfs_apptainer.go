// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build apptainer_engine

package unpacker

import (
	"bufio"
	"bytes"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/sylog"
)

func init() {
	cmdFunc = unsquashfsSandboxCmd
}

// libBind represents a library bind mount required by an elf binary
// that will be run in a contained minimal filesystem.
type libBind struct {
	// source is the path to bind from, on the host.
	source string
	// dest is the path to bind to, inside the minimal filesystem.
	dest string
}

// getLibraryBinds returns the library bind mounts required by an elf binary.
// The binary path must be absolute.
func getLibraryBinds(binary string) ([]libBind, error) {
	var wrapper string
	exe, err := elf.Open(binary)
	if err != nil {
		if !strings.Contains(err.Error(), "bad magic number") {
			return nil, fmt.Errorf("error opening elf binary %s: %v", binary, err)
		}

		// Likely to instead be a relocating wrapper script.
		// If it was generated with tools/install-unprivileged.sh,
		// it will be able to replace the usual "exec" with the
		// value of "$_WRAPPER_EXEC_CMD", so try that.
		wrapper = binary
		cmd := exec.Command(wrapper)
		buf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		cmd.Stdout = buf
		cmd.Stderr = errBuf
		cmd.Env = []string{"_WRAPPER_EXEC_CMD=echo"}
		sylog.Debugf("Running %s %s", cmd.Env[0], wrapper)
		if err = cmd.Run(); err != nil {
			if errBuf.Len() > 0 {
				sylog.Debugf("stderr was: %s", errBuf.String())
			}
			return nil, fmt.Errorf("while running %s %s: %s", cmd.Env[0], wrapper, err)
		}
		binary = buf.String()
		if strings.Count(binary, "\n") != 1 {
			sylog.Debugf("stdout was: %s", binary)
			return nil, fmt.Errorf("did not receive exactly one line from %s %s", cmd.Env[0], wrapper)
		}
		binary = strings.TrimSpace(binary)
		exe, err = elf.Open(binary)
		if err != nil {
			return nil, fmt.Errorf("error opening elf binary %s: %v", binary, err)
		}
	}
	defer exe.Close()

	interp := ""

	// look for the interpreter
	for _, p := range exe.Progs {
		if p.Type != elf.PT_INTERP {
			continue
		}
		buf := make([]byte, 4096)
		n, err := p.ReadAt(buf, 0)
		if err != nil && err != io.EOF {
			return nil, err
		} else if n > cap(buf) {
			return nil, fmt.Errorf("buffer too small to store interpreter")
		}
		// trim null byte to avoid an execution failure with
		// an invalid argument error
		interp = string(bytes.Trim(buf, "\x00"))
	}

	// this is a static binary, nothing to do
	if interp == "" {
		return []libBind{}, nil
	}

	// run interpreter to list library dependencies for the
	// corresponding binary, eg:
	// /lib64/ld-linux-x86-64.so.2 --list <program>
	// /lib/ld-musl-x86_64.so.1 --list <program>
	errBuf := new(bytes.Buffer)
	buf := new(bytes.Buffer)

	// set an empty environment as LD_LIBRARY_PATH
	// may mix dependencies, just rely only on the library
	// cache or its own lookup mechanism, see issue:
	// https://github.com/apptainer/singularity/issues/5666
	env := []string{}

	var cmd *exec.Cmd
	if wrapper != "" {
		env = []string{"_WRAPPER_EXEC_CMD=" + interp + " --list"}
		cmd = exec.Command(wrapper)
		sylog.Debugf("Running %s %s", strings.Replace(env[0], "=", "=\"", 1)+"\"", wrapper)
	} else {
		cmd = exec.Command(interp, "--list", binary)
		sylog.Debugf("Running %s --list %s", interp, binary)
	}
	cmd.Stdout = buf
	cmd.Stderr = errBuf
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("while getting library dependencies: %s\n%s", err, errBuf.String())
	}

	binds, err := parseLibraryBinds(buf)
	if err != nil {
		return binds, err
	}
	if wrapper != "" {
		// Add the wrapped binary, bash, and new libraries used by
		//  bash to the library binds
		binds = append(binds, []libBind{
			{
				source: binary,
				dest:   binary,
			},
			{
				source: "/bin/bash",
				dest:   "/bin/bash",
			},
		}...)
		bashbinds, err := getLibraryBinds("/bin/bash")
		if err != nil {
			return nil, fmt.Errorf("error getting libraries of /bin/bash: %v", err)
		}
		for _, newbind := range bashbinds {
			gotone := false
			for _, oldbind := range binds {
				if oldbind.dest == newbind.dest {
					gotone = true
					break
				}
			}
			if !gotone {
				binds = append(binds, newbind)
			}
		}
	}
	return binds, nil
}

// parseLibrary binds parses `ld-linux-x86-64.so.2 --list <binary>` output.
// Returns a list of source->dest bind mounts required to run the binary
// in a minimal contained filesystem.
func parseLibraryBinds(buf io.Reader) ([]libBind, error) {
	libs := make([]libBind, 0)
	scanner := bufio.NewScanner(buf)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		// /lib64/ld64.so.2 (0x00007fff96c60000)
		// Absolute path in 1st field - bind directly dest=source
		if filepath.IsAbs(fields[0]) {
			libs = append(libs, libBind{
				source: fields[0],
				dest:   fields[0],
			})
			continue
		}
		// libpthread.so.0 => /lib64/libpthread.so.0 (0x00007fff96a20000)
		//    .. or with glibc-hwcaps ..
		// libpthread.so.0 => /lib64/glibc-hwcaps/power9/libpthread-2.28.so (0x00007fff96a20000)
		//
		// Bind resolved lib to same dir, but with .so filename from 1st field.
		// e.g. source is: /lib64/glibc-hwcaps/power9/libpthread-2.28.so
		//      dest is  : /lib64/glibc-hwcaps/power9/libpthread.so.0
		if len(fields) >= 3 && fields[1] == "=>" && filepath.IsAbs(fields[2]) {
			destDir := filepath.Dir(fields[2])
			dest := filepath.Join(destDir, fields[0])
			libs = append(libs, libBind{
				source: fields[2],
				dest:   dest,
			})
		}
		// linux-vdso64.so.1 (0x00007fff96c40000)
		// linux-vdso64.so.1 =>  (0x00007fff96c40000)
		//   .. or anything else
		// No absolute path = nothing to bind
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("while parsing library dependencies: %v", err)
	}

	return libs, nil
}

// unsquashfsSandboxCmd is the command instance for executing unsquashfs command
// in a sandboxed environment with apptainer.
func unsquashfsSandboxCmd(unsquashfs string, dest string, filename string, filter string, opts ...string) (*exec.Cmd, error) {
	const (
		// will contain both dest and filename inside the sandbox
		rootfsImageDir = "/image"
	)

	// create the sandbox temporary directory
	tmpdir := filepath.Dir(dest)
	rootfs, err := os.MkdirTemp(tmpdir, "tmp-rootfs-")
	if err != nil {
		return nil, fmt.Errorf("failed to create chroot directory: %s", err)
	}

	overwrite := false

	// remove the destination directory if any, if the directory is
	// not empty (typically during image build), the unsafe option -f is
	// set, this is unfortunately required by image build
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to remove %s: %s", dest, err)
		}
		overwrite = true
	}

	// map destination into the sandbox
	rootfsDest := filepath.Join(rootfsImageDir, filepath.Base(dest))

	// sandbox required directories
	rootfsDirs := []string{
		// unsquashfs get available CPU from /sys/devices/system/cpu/online
		filepath.Join(rootfs, "/sys"),
		filepath.Join(rootfs, "/dev"),
		// If unsquashfs has a relative RPATH the ELF interpreter needs /proc
		filepath.Join(rootfs, "/proc"),
		filepath.Join(rootfs, rootfsImageDir),
	}

	for _, d := range rootfsDirs {
		if err := os.Mkdir(d, 0o700); err != nil {
			return nil, fmt.Errorf("while creating %s: %s", d, err)
		}
	}

	// the decision to use user namespace is left to apptainer
	// which will detect automatically depending of the configuration
	// what workflow it could use
	args := []string{
		"exec",
		"--no-home",
		"--no-nv",
		"--no-rocm",
		"-C",
		"--no-init",
		"--writable",
		"-B", fmt.Sprintf("%s:%s", tmpdir, rootfsImageDir),
	}

	if filename != stdinFile {
		filename = filepath.Join(rootfsImageDir, filepath.Base(filename))
	}

	roFiles := []string{
		unsquashfs,
	}

	// get the library dependencies of unsquashfs
	libs, err := getLibraryBinds(unsquashfs)
	if err != nil {
		return nil, err
	}

	// Handle binding of files
	for _, b := range roFiles {
		// Ensure parent dir and file exist in container
		rootfsFile := filepath.Join(rootfs, b)
		rootfsDir := filepath.Dir(rootfsFile)
		if err := os.MkdirAll(rootfsDir, 0o700); err != nil {
			return nil, fmt.Errorf("while creating %s: %s", rootfsDir, err)
		}
		if err := os.WriteFile(rootfsFile, []byte(""), 0o600); err != nil {
			return nil, fmt.Errorf("while creating %s: %s", rootfsFile, err)
		}
		// Simple read-only bind, dest in container same as source on host
		args = append(args, "-B", fmt.Sprintf("%s:%s:ro", b, b))
	}

	// Handle binding of libs and generate LD_LIBRARY_PATH
	libraryPath := strings.Split(os.Getenv("LD_LIBRARY_PATH"), string(os.PathListSeparator))
	for _, l := range libs {
		// Ensure parent dir and file exist in container
		rootfsFile := filepath.Join(rootfs, l.dest)
		rootfsDir := filepath.Dir(rootfsFile)
		if err := os.MkdirAll(rootfsDir, 0o700); err != nil {
			return nil, fmt.Errorf("while creating %s: %s", rootfsDir, err)
		}
		if err := os.WriteFile(rootfsFile, []byte(""), 0o600); err != nil {
			return nil, fmt.Errorf("while creating %s: %s", rootfsFile, err)
		}
		// Read only bind, dest in container may not match source on host due
		// to .so symlinking (see getLibraryBinds comments).
		args = append(args, "-B", fmt.Sprintf("%s:%s:ro", l.source, l.dest))
		// If dir of lib not already in the LD_LIBRARY_PATH, add it.
		has := false
		libraryDir := filepath.Dir(l.dest)
		for _, lp := range libraryPath {
			if lp == libraryDir {
				has = true
				break
			}
		}
		if !has {
			libraryPath = append(libraryPath, libraryDir)
		}
	}

	// apptainer sandbox
	args = append(args, rootfs)

	// unsquashfs execution arguments
	args = append(args, unsquashfs)
	args = append(args, opts...)

	if overwrite {
		args = append(args, "-f")
	}

	args = append(args, "-d", rootfsDest, filename)

	if filter != "" {
		args = append(args, filter)
	}

	sylog.Debugf("Calling wrapped unsquashfs: apptainer %v", args)
	sylog.Debugf("LD_LIBRARY_PATH=%v", strings.Join(libraryPath, string(os.PathListSeparator)))
	cmd := exec.Command(filepath.Join(buildcfg.BINDIR, "apptainer"), args...)
	cmd.Dir = "/"
	cmd.Env = []string{
		fmt.Sprintf("LD_LIBRARY_PATH=%s", strings.Join(libraryPath, string(os.PathListSeparator))),
		fmt.Sprintf("APPTAINERENV_LD_LIBRARY_PATH=%s", strings.Join(libraryPath, string(os.PathListSeparator))),
		fmt.Sprintf("APPTAINER_DEBUG=%s", os.Getenv("APPTAINER_DEBUG")),
		fmt.Sprintf("APPTAINER_VERBOSE=%s", os.Getenv("APPTAINER_VERBOSE")),
		fmt.Sprintf("APPTAINER_QUIET=%s", os.Getenv("APPTAINER_QUIET")),
		fmt.Sprintf("APPTAINER_SILENT=%s", os.Getenv("APPTAINER_SILENT")),
		fmt.Sprintf("APPTAINER_MESSAGELEVEL=%s", os.Getenv("APPTAINER_MESSAGELEVEL")),
	}

	return cmd, nil
}
