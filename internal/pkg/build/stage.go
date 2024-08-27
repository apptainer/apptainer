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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/build/files"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/fakeroot"
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

// runHostScript executes the stage's pre or setup script on host.
func (s *stage) runHostScript(name string, script types.Script) error {
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
			// Bind the fakeroot components.  Once they are there,
			//  the nested apptainer will run fakeroot if it isn't
			//  started and pass down the components and environment
			//  to nested apptainers.
			fakerootBinds, err = fakeroot.GetFakeBinds(s.b.Opts.FakerootPath)
			if err != nil {
				return fmt.Errorf("while getting fakeroot bindpoints: %v", err)
			}
			err = s.makeFakerootBindpoints(fakerootBinds)
			if err != nil {
				return fmt.Errorf("while creating fakeroot bindpoints: %v", err)
			}
			cmdArgs = append(cmdArgs, "-B", strings.Join(fakerootBinds[:], ","))
		}
		if len(s.b.Opts.Binds) != 0 {
			for _, bind := range s.b.Opts.Binds {
				cmdArgs = append(cmdArgs, "-B", bind)
			}
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

		env := currentEnvNoApptainer([]string{"DEBUG", "NV", "NVCCLI", "ROCM", "BINDPATH", "MOUNT"})
		cmdArgs = append(cmdArgs, s.b.RootfsPath)
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
		if err != nil {
			return fmt.Errorf("while running %%post section: %s", err)
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
		if len(s.b.Opts.Binds) != 0 {
			for _, bind := range s.b.Opts.Binds {
				cmdArgs = append(cmdArgs, "-B", bind)
			}
		}

		exe := filepath.Join(buildcfg.BINDIR, "apptainer")

		cmdArgs = append(cmdArgs, s.b.RootfsPath)
		cmd := exec.Command(exe, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = "/"
		cmd.Env = currentEnvNoApptainer([]string{"DEBUG", "NV", "NVCCLI", "ROCM", "BINDPATH", "MOUNT", "WRITABLE_TMPFS"})

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

func (s *stage) makeFakerootBindpoints(binds []string) error {
	for _, bind := range binds {
		splits := strings.Split(bind, ":")
		point := splits[len(splits)-1]
		sylog.Debugf("Making %v mount point", point)
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
	}
	return nil
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
