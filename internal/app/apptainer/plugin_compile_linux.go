// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/plugin"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/sif/v2/pkg/sif"
)

// source file that should be present in a valid Apptainer source tree
const canaryFile = "pkg/plugin/plugin.go"

const goVersionFile = `package main
import "fmt"
import "runtime"
func main() { fmt.Printf(runtime.Version()) }`

type buildToolchain struct {
	workPath  string
	goPath    string
	buildTags string
	envs      []string
}

// getApptainerSrcDir returns the source directory for apptainer.
func getApptainerSrcDir() (string, error) {
	if buildcfg.IsReproducibleBuild() {
		return "", fmt.Errorf("plugin functionality is not available in --reproducible builds of apptainer")
	}

	dir := buildcfg.SOURCEDIR
	canary := filepath.Join(dir, canaryFile)
	sylog.Debugf("Searching source file %s", canary)

	switch _, err := os.Stat(canary); {
	case os.IsNotExist(err):
		return "", fmt.Errorf("cannot find %q", canary)

	case err != nil:
		return "", fmt.Errorf("unexpected error while looking for %q: %s", canary, err)

	default:
		return dir, nil
	}
}

// checkGoVersion returns an error if the currently Go toolchain is
// different from the one used to compile apptainer. Apptainer
// and plugin must be compiled with the same toolchain.
func checkGoVersion(goPath string) error {
	var out bytes.Buffer

	tmpDir, err := os.MkdirTemp("", "plugin-")
	if err != nil {
		return errors.New("temporary directory creation failed")
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "rt_version.go")
	if err := os.WriteFile(path, []byte(goVersionFile), 0o600); err != nil {
		return fmt.Errorf("while writing go file %s: %s", path, err)
	}
	defer os.Remove(path)

	cmd := exec.Command(goPath, "run", path)
	cmd.Dir = tmpDir
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while executing go version: %s", err)
	}

	output := out.String()

	runtimeVersion := runtime.Version()
	if output != runtimeVersion {
		return fmt.Errorf("plugin compilation requires Go runtime %q, current is %q", runtimeVersion, output)
	}

	return nil
}

// pluginObjPath returns the path of the .so file which is built when
// running `go build -buildmode=plugin [...]`.
func pluginObjPath(sourceDir string) string {
	return filepath.Join(sourceDir, "plugin.so")
}

// pluginManifestPath returns the path of the .manifest file created
// in the container after the plugin object is built
func pluginManifestPath(sourceDir string) string {
	return filepath.Join(sourceDir, "plugin.manifest")
}

// CompilePlugin compiles a plugin. It takes as input: sourceDir, the path to the
// plugin's source code directory; and destSif, the path to the intended final
// location of the plugin SIF file.
func CompilePlugin(sourceDir, destSif, buildTags string) error {
	apptainerSrc, err := getApptainerSrcDir()
	if err != nil {
		return fmt.Errorf("apptainer source directory not usable: %w", err)
	}
	apptainerSrc, err = filepath.Abs(apptainerSrc)
	if err != nil {
		return fmt.Errorf("while getting absolute path of %q: %w", apptainerSrc, err)
	}
	sylog.Debugf("Using apptainer source: %s", apptainerSrc)
	pluginSrc, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("while getting absolute path of %q: %w", pluginSrc, err)
	}
	sylog.Debugf("Using plugin source: %s", pluginSrc)
	if !strings.HasPrefix(pluginSrc, apptainerSrc+string(os.PathSeparator)) {
		return fmt.Errorf("plugin source %q must be inside apptainer source %q", pluginSrc, apptainerSrc)
	}

	goPath, err := bin.FindBin("go")
	if err != nil {
		return errors.New("go compiler not found")
	}

	// we need to use the exact same go runtime version used
	// to compile Apptainer
	if err := checkGoVersion(goPath); err != nil {
		return fmt.Errorf("while checking go version: %s", err)
	}

	bTool := buildToolchain{
		buildTags: buildTags,
		workPath:  apptainerSrc,
		goPath:    goPath,
		envs:      append(os.Environ(), "GO111MODULE=on"),
	}

	// build plugin object using go build
	soPath, err := buildPlugin(pluginSrc, bTool)
	if err != nil {
		return fmt.Errorf("while building plugin .so: %v", err)
	}
	defer os.Remove(soPath)

	// generate plugin manifest from .so
	mPath, err := generateManifest(pluginSrc, bTool)
	if err != nil {
		return fmt.Errorf("while generating plugin manifest: %s", err)
	}
	defer os.Remove(mPath)

	// convert the built plugin object into a sif
	if err := makeSIF(pluginSrc, destSif); err != nil {
		return fmt.Errorf("while making sif file: %s", err)
	}

	sylog.Infof("Plugin built to: %s", destSif)

	return nil
}

// buildPlugin takes sourceDir which is the string path the host which
// contains the source code of the plugin. buildPlugin returns the path
// to the built file, along with an error.
//
// This function essentially runs the `go build -buildmode=plugin [...]`
// command.
func buildPlugin(sourceDir string, bTool buildToolchain) (string, error) {
	// assuming that sourceDir is within trimpath for now
	out := pluginObjPath(sourceDir)
	// set pluginRootDirVar variable if required by the plugin
	pluginRootDirVar := fmt.Sprintf("-X main.%s=%s", pluginapi.PluginRootDirSymbol, buildcfg.PLUGIN_ROOTDIR)

	args := []string{
		"build",
		"-a",
		"-o", out,
		"-mod=readonly",
		"-ldflags", pluginRootDirVar,
		"-trimpath",
		"-buildmode=plugin",
		"-tags", bTool.buildTags,
		sourceDir,
	}

	sylog.Debugf("Running: %s %s", bTool.goPath, strings.Join(args, " "))

	buildcmd := exec.Command(bTool.goPath, args...)

	buildcmd.Dir = bTool.workPath
	buildcmd.Stderr = os.Stderr
	buildcmd.Stdout = os.Stdout
	buildcmd.Stdin = os.Stdin
	buildcmd.Env = bTool.envs

	return out, buildcmd.Run()
}

// generateManifest takes the path to the plugin source, extracts
// plugin's manifest by loading it into memory and stores it's json
// representation in a separate file.
func generateManifest(sourceDir string, _ buildToolchain) (string, error) {
	in := pluginObjPath(sourceDir)
	out := pluginManifestPath(sourceDir)

	p, err := plugin.LoadObject(in)
	if err != nil {
		return "", fmt.Errorf("while loading plugin %s: %s", in, err)
	}

	f, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("while creating manifest %s: %s", out, err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(p.Manifest); err != nil {
		return "", fmt.Errorf("while writing manifest %s: %s", out, err)
	}

	return out, nil
}

// makeSIF takes in two arguments: sourceDir, the path to the plugin source directory;
// and sifPath, the path to the final .sif file which is ready to be used.
func makeSIF(sourceDir, sifPath string) error {
	objPath := pluginObjPath(sourceDir)

	fp, err := os.Open(objPath)
	if err != nil {
		return fmt.Errorf("while opening plugin object file %v: %w", objPath, err)
	}
	defer fp.Close()

	plObjInput, err := sif.NewDescriptorInput(sif.DataPartition, fp,
		sif.OptObjectName("plugin.so"),
		sif.OptPartitionMetadata(sif.FsRaw, sif.PartData, runtime.GOARCH),
	)
	if err != nil {
		return err
	}

	// create plugin manifest descriptor
	manifestPath := pluginManifestPath(sourceDir)

	fp, err = os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("while opening plugin manifest file %v: %w", manifestPath, err)
	}
	defer fp.Close()

	plManifestInput, err := sif.NewDescriptorInput(sif.DataGenericJSON, fp,
		sif.OptObjectName("plugin.manifest"),
	)
	if err != nil {
		return err
	}

	os.RemoveAll(sifPath)

	f, err := sif.CreateContainerAtPath(sifPath,
		sif.OptCreateWithDescriptors(plObjInput, plManifestInput),
	)
	if err != nil {
		return fmt.Errorf("while creating sif file: %w", err)
	}

	if err := f.UnloadContainer(); err != nil {
		return fmt.Errorf("while unloading sif file: %w", err)
	}

	return nil
}
