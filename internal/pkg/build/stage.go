// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package build

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/build/files"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	envUtil "github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// stage represents the process of constructing a root filesystem.
type stage struct {
	// name of the stage.
	name string
	// c Gets and Packs data needed to build a container into a Bundle from various sources.
	c ConveyorPacker
	// a Assembles a container from the information stored in a Bundle into various formats.
	a Assembler
	// b is an intermediate structure that encapsulates all information for the container, e.g., metadata, filesystems.
	b *types.Bundle
}

const (
	sLabelsPath  = "/.build.labels"
	aEnvironment = "APPTAINER_ENVIRONMENT=/.singularity.d/env/91-environment.sh"
	sEnvironment = "SINGULARITY_ENVIRONMENT=/.singularity.d/env/91-environment.sh"
	aLabels      = "APPTAINER_LABELS=" + sLabelsPath
	sLabels      = "SINGULARITY_LABELS=" + sLabelsPath
)

// Assemble assembles the bundle to the specified path.
func (s *stage) Assemble(path string) error {
	return s.a.Assemble(s.b, path)
}

// runSectionScript executes the stage's pre and setup scripts on host.
func (s *stage) runSectionScript(name string, script types.Script) error {
	if s.b.RunSection(name) && script.Script != "" {
		aRootfs := "APPTAINER_ROOTFS=" + s.b.RootfsPath
		sRootfs := "SINGULARITY_ROOTFS=" + s.b.RootfsPath

		scriptPath := filepath.Join(s.b.TmpDir, name)
		if err := createScript(scriptPath, []byte(script.Script)); err != nil {
			return fmt.Errorf("while creating %s script: %s", name, err)
		}
		defer os.Remove(scriptPath)

		args, err := getSectionScriptArgs(name, scriptPath, script)
		if err != nil {
			return fmt.Errorf("while processing section %%%s arguments: %s", name, err)
		}

		// Run script section here
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, aEnvironment, sEnvironment, aRootfs, sRootfs)

		sylog.Infof("Running %s scriptlet", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %%%s script: %v", name, err)
		}
	}
	return nil
}

func (s *stage) runPostScript(sessionResolv, sessionHosts string) error {
	if s.b.Recipe.BuildData.Post.Script != "" {
		cmdArgs := []string{"-s", "--build-config", "exec", "--pwd", "/", "--writable"}
		cmdArgs = append(cmdArgs, "--cleanenv", "--env", aEnvironment, "--env", sEnvironment, "--env", aLabels, "--env", sLabels)

		if sessionResolv != "" {
			cmdArgs = append(cmdArgs, "-B", sessionResolv+":/etc/resolv.conf")
		}
		if sessionHosts != "" {
			cmdArgs = append(cmdArgs, "-B", sessionHosts+":/etc/hosts")
		}
		var fakerootBinds []string
		var err error
		if s.b.Opts.FakerootPath != "" {
			fakerootBinds, err = s.makeFakerootBindpoints()
			if err != nil {
				return fmt.Errorf("while creating fakeroot bindpoints: %v", err)
			}
			cmdArgs = append(cmdArgs, "-B", strings.Join(fakerootBinds[:], ","))
		}

		script := s.b.Recipe.BuildData.Post
		scriptPath := filepath.Join(s.b.RootfsPath, ".post.script")
		if err = createScript(scriptPath, []byte(script.Script)); err != nil {
			return fmt.Errorf("while creating post script: %s", err)
		}
		defer os.Remove(scriptPath)

		args, err := getSectionScriptArgs("post", "/.post.script", script)
		if err != nil {
			return fmt.Errorf("while processing section %%post arguments: %s", err)
		}

		exe := filepath.Join(buildcfg.BINDIR, "apptainer")

		env := currentEnvNoApptainer([]string{"NV", "NVCCLI", "ROCM", "BINDPATH", "MOUNT"})
		cmdArgs = append(cmdArgs, s.b.RootfsPath)
		if s.b.Opts.FakerootPath != "" {
			base := filepath.Base(s.b.Opts.FakerootPath)
			sylog.Debugf("Post scriptlet will be run with %v", base)
			cmdArgs = append(cmdArgs, "/usr/bin/"+base)
			// Without this workaround fakeroot does not work
			//  properly in a user namespace. It is especially
			//  noticeable with debian containers.  Learned from
			//  https://salsa.debian.org/clint/fakeroot/-/merge_requests/4
			env = append(env, envUtil.ApptainerEnvPrefix+"FAKEROOTDONTTRYCHOWN=1")
		}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command(exe, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = "/"
		cmd.Env = env

		sylog.Infof("Running post scriptlet")
		err = cmd.Run()
		if len(fakerootBinds) > 0 {
			s.cleanFakerootBindpoints(fakerootBinds)
		}
		return err
	}
	return nil
}

func (s *stage) runTestScript(sessionResolv, sessionHosts string) error {
	if !s.b.Opts.NoTest && s.b.Recipe.BuildData.Test.Script != "" {
		cmdArgs := []string{"-s", "--build-config", "test", "--pwd", "/"}

		if sessionResolv != "" {
			cmdArgs = append(cmdArgs, "-B", sessionResolv+":/etc/resolv.conf")
		}
		if sessionHosts != "" {
			cmdArgs = append(cmdArgs, "-B", sessionHosts+":/etc/hosts")
		}

		exe := filepath.Join(buildcfg.BINDIR, "apptainer")

		cmdArgs = append(cmdArgs, s.b.RootfsPath)
		cmd := exec.Command(exe, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = "/"
		cmd.Env = currentEnvNoApptainer([]string{"NV", "NVCCLI", "ROCM", "BINDPATH", "MOUNT", "WRITABLE_TMPFS"})

		sylog.Infof("Running testscript")
		return cmd.Run()
	}
	return nil
}

func (s *stage) copyFilesFrom(b *Build) error {
	def := s.b.Recipe
	for _, f := range def.BuildData.Files {
		// Trim comments from args
		cleanArgs := strings.Split(f.Args, "#")[0]
		if cleanArgs == "" {
			continue
		}

		args := strings.Fields(cleanArgs)
		if len(args) != 2 {
			continue
		}

		stageIndex, err := b.findStageIndex(args[1])
		if err != nil {
			return err
		}

		srcRootfsPath := b.stages[stageIndex].b.RootfsPath
		dstRootfsPath := s.b.RootfsPath

		sylog.Debugf("Copying files from stage: %s", args[1])

		// iterate through filetransfers
		for _, transfer := range f.Files {
			// sanity
			if transfer.Src == "" {
				sylog.Warningf("Attempt to copy file with no name, skipping.")
				continue
			}
			// copy each file into bundle rootfs
			sylog.Infof("Copying %v to %v", transfer.Src, transfer.Dst)
			if err := files.CopyFromStage(transfer.Src, transfer.Dst, srcRootfsPath, dstRootfsPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *stage) copyFiles() error {
	def := s.b.Recipe
	filesSection := types.Files{}
	for _, f := range def.BuildData.Files {
		// Trim comments from args
		cleanArgs := strings.Split(f.Args, "#")[0]
		if cleanArgs == "" {
			filesSection.Files = append(filesSection.Files, f.Files...)
		}
	}
	// iterate through filetransfers
	for _, transfer := range filesSection.Files {
		// sanity
		if transfer.Src == "" {
			sylog.Warningf("Attempt to copy file with no name, skipping.")
			continue
		}
		// copy each file into bundle rootfs
		sylog.Infof("Copying %v to %v", transfer.Src, transfer.Dst)
		if err := files.CopyFromHost(transfer.Src, transfer.Dst, s.b.RootfsPath); err != nil {
			return err
		}
	}

	return nil
}

// make a file mountpoint
// assumes directory already exists
func (s *stage) makeFilePoint(point string) error {
	sylog.Debugf("Making file mountpoint %v", point)
	path := filepath.Join(s.b.RootfsPath, point)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("while making file mountpoint %v: %v", point, err)
		}
		file.Close()
	} else if err != nil {
		return fmt.Errorf("while making file mountpoint %v: %v", point, err)
	}
	return nil
}

// make a directory mountpoint
// will create parent directory if missing
func (s *stage) makeDirPoint(point string) error {
	sylog.Debugf("Making directory mountpoint %v", point)
	path := filepath.Join(s.b.RootfsPath, point)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.MkdirAll(path, 0o755)
		if err != nil {
			return fmt.Errorf("while making directory mountpoint %v: %v", point, err)
		}
	} else if err != nil {
		return fmt.Errorf("while making directory mountpoint %v: %v", point, err)
	}
	return nil
}

func (s *stage) makeFakerootBindpoints() ([]string, error) {
	var binds []string

	// Start by examining the environment fakeroot creates
	cmd := exec.Command(s.b.Opts.FakerootPath, "env")
	env := os.Environ()
	for idx := range env {
		if strings.HasPrefix(env[idx], "LD_LIBRARY_PATH=") {
			// Remove any incoming LD_LIBRARY_PATH
			env[idx] = "LD_LIBRARY_PREFIX="
		}
	}
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return binds, fmt.Errorf("error make fakeroot stdout pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		return binds, fmt.Errorf("error starting fakeroot: %v", err)
	}
	preload := ""
	libraryPath := ""
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "LD_PRELOAD=") {
			preload = line[len("LD_PRELOAD="):]
		} else if strings.HasPrefix(line, "LD_LIBRARY_PATH=") {
			libraryPath = line[len("LD_LIBRARY_PATH="):]
		}
	}
	_ = cmd.Wait()
	if preload == "" {
		return binds, fmt.Errorf("No LD_PRELOAD in fakeroot environment")
	}
	if libraryPath == "" {
		return binds, fmt.Errorf("No LD_LIBRARY_PATH in fakeroot environment")
	}

	src := s.b.Opts.FakerootPath
	point := "/usr/bin/fakeroot"
	err = s.makeFilePoint(point)
	if err != nil {
		return binds, err
	}
	binds = append(binds, src+":"+point)

	// faked isn't strictly needed but include it if present in case it
	// is used in a future version
	dir := filepath.Dir(src)
	src = filepath.Join(dir, "faked")
	point = "/usr/bin/faked"
	if _, err = os.Stat(src); err == nil {
		err = s.makeFilePoint(point)
		if err != nil {
			return binds, err
		}
		binds = append(binds, src+":"+point)
	}
	splits := strings.Split(preload, ".")
	splits = strings.Split(splits[0], "-")
	if len(splits) > 1 {
		// add the faked that corresponds to the preload library
		src += "-" + splits[1]
		point += "-" + splits[1]
		if _, err = os.Stat(src); err == nil {
			err = s.makeFilePoint(point)
			if err != nil {
				return binds, err
			}
			binds = append(binds, src+":"+point)
		}
	}
	splits = strings.Split(libraryPath, ":")
	for _, dir := range splits {
		// Find the directory in libraryPath that contains the
		// preload library and include that
		src = filepath.Join(dir, preload)
		if _, err = os.Stat(src); err == nil {
			err = s.makeDirPoint(dir)
			if err != nil {
				return binds, err
			}
			binds = append(binds, dir+"/")
			break
		}
	}
	return binds, nil
}

func (s *stage) cleanFakerootBindpoints(binds []string) {
	for _, bind := range binds {
		splits := strings.Split(bind, ":")
		point := splits[len(splits)-1]
		sylog.Debugf("Removing %v mount point", point)
		path := filepath.Join(s.b.RootfsPath, point)
		if strings.HasSuffix(point, "/") {
			// remove parent directories until not empty
			for {
				if err := syscall.Rmdir(path); err != nil {
					sylog.Debugf("Removing %v did not succeed because: %v", path[len(s.b.RootfsPath):], err)
					break
				}
				path = filepath.Dir(path)
				sylog.Debugf("Attempting to remove %v", path[len(s.b.RootfsPath):])
			}
		} else if fileinfo, err := os.Stat(path); err == nil {
			if fileinfo.Size() == 0 {
				syscall.Unlink(path)
			}
		}
	}
}
