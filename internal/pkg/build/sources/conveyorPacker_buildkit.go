// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2024, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	build_oci "github.com/apptainer/apptainer/internal/pkg/build/oci"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/internal/pkg/ociplatform"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// BuildKitConveyorPacker only needs to hold the conveyor to have the needed data to pack
type BuildKitConveyorPacker struct {
	OCIConveyorPacker
	context   string
	frontend  string
	filename  string
}

// Get just stores the source
func (cp *BuildKitConveyorPacker) Get(_ context.Context, b *types.Bundle) (err error) {
	cp.b = b

	cp.topts = &ociimage.TransportOptions{
		AuthConfig:   cp.b.Opts.OCIAuthConfig,
		AuthFilePath: ociauth.ChooseAuthFile(cp.b.Opts.ReqAuthFile),
		TmpDir:       b.TmpDir,
		Platform:     cp.b.Opts.Platform,
	}

	if cp.b.Opts.OCIAuthConfig == nil && cp.b.Opts.DockerAuthConfig != nil {
		cp.topts.AuthConfig = &authn.AuthConfig{
			Username:      cp.b.Opts.DockerAuthConfig.Username,
			Password:      cp.b.Opts.DockerAuthConfig.Password,
			IdentityToken: cp.b.Opts.DockerAuthConfig.IdentityToken,
		}
	}

	dp, err := ociplatform.DefaultPlatform()
	if err != nil {
		return err
	}
	cp.topts.Platform = *dp

	if cp.b.Opts.Arch != "" {
		if arch, ok := build_oci.ArchMap[cp.b.Opts.Arch]; ok {
			cp.topts.Platform = v1.Platform{
				OS:           dp.OS,
				Architecture: arch.Arch,
				Variant:      arch.Var,
			}
		} else {
			keys := reflect.ValueOf(build_oci.ArchMap).MapKeys()
			return fmt.Errorf("failed to parse the arch value: %s, should be one of %v", cp.b.Opts.Arch, keys)
		}
	}
	sylog.Debugf("Platform: %s", cp.topts.Platform)

	cp.context = b.Recipe.Header["from"]
	_, err = os.Stat(cp.context)
	if err != nil {
		return err
	}
	cp.frontend = b.Recipe.Header["frontend"]
	if cp.frontend == "" {
		cp.frontend = "dockerfile.v0"
	}
	cp.filename = b.Recipe.Header["filename"]
	if cp.filename == "" {
		cp.filename = "Dockerfile"
	}

	return nil
}

// Pack puts relevant objects in a Bundle!
func (cp *BuildKitConveyorPacker) Pack(ctx context.Context) (b *types.Bundle, err error) {
	sylog.Infof("Building OCI image...")
	err = cp.buildImage(ctx)
	if err != nil {
		return nil, fmt.Errorf("while building image: %v", err)
	}

	sylog.Infof("Extracting OCI image...")
	err = cp.unpackRootfs(ctx)
	if err != nil {
		return nil, fmt.Errorf("while unpacking rootfs: %v", err)
	}

	sylog.Infof("Inserting Apptainer configuration...")
	err = cp.insertBaseEnv()
	if err != nil {
		return nil, fmt.Errorf("while inserting base environment: %v", err)
	}

	err = cp.insertRunScript()
	if err != nil {
		return nil, fmt.Errorf("while inserting runscript: %v", err)
	}

	err = cp.insertEnv()
	if err != nil {
		return nil, fmt.Errorf("while inserting docker specific environment: %v", err)
	}

	err = cp.insertOCIConfig()
	if err != nil {
		return nil, fmt.Errorf("while inserting oci config: %v", err)
	}

	err = cp.insertOCILabels()
	if err != nil {
		return nil, fmt.Errorf("while inserting oci labels: %v", err)
	}

	return cp.b, nil
}

func (cp *BuildKitConveyorPacker) buildImage(ctx context.Context) error {
	tmpfile, err := os.CreateTemp("/var/tmp", "buildkit-*.tar")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	platform := cp.topts.Platform.String()

	buildargs := []string{
		"build",
		fmt.Sprintf("--frontend=%s", cp.frontend),
		"--local", fmt.Sprintf("context=%s", cp.context),
		"--local", fmt.Sprintf("dockerfile=%s", cp.context),
		"--opt", fmt.Sprintf("filename=%s", cp.filename),
		"--opt", fmt.Sprintf("platform=%s", platform),
		"--output", fmt.Sprintf("type=oci,dest=%s", tmpfile.Name()),
	}
	cmd := exec.CommandContext(ctx, "buildctl", buildargs...)
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	var imgCache *cache.Handle

	ref := "oci-archive:" + tmpfile.Name()

	// Fetch the image into a temporary containers/image oci layout dir.
	cp.srcImg, err = ociimage.FetchToLayout(ctx, cp.topts, imgCache, ref, cp.b.TmpDir)
	if err != nil {
		return err
	}

	cf, err := cp.srcImg.ConfigFile()
	if err != nil {
		return err
	}
	cp.imgConfig = cf.Config

	return nil
}
