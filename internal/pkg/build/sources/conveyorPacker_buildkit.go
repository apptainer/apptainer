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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

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

type buildkitOptions struct {
	context   string
	target    string
	frontend  string
	filename  string
	buildargs map[string]string
}

// BuildKitConveyorPacker only needs to hold the conveyor to have the needed data to pack
type BuildKitConveyorPacker struct {
	OCIConveyorPacker
	bk buildkitOptions
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

	bk := &cp.bk

	bk.context = b.Recipe.Header["from"]
	_, err = os.Stat(bk.context)
	if err != nil {
		return err
	}
	bk.target = b.Recipe.Header["target"]
	bk.frontend = b.Recipe.Header["frontend"]
	if bk.frontend == "" {
		bk.frontend = "dockerfile.v0"
	}
	bk.filename = b.Recipe.Header["filename"]
	if bk.filename == "" {
		bk.filename = "Dockerfile"
	}
	bk.buildargs = map[string]string{}
	args := b.Recipe.Header["buildargs"]
	for _, pair := range strings.Split(args, " ") {
		fields := strings.SplitN(pair, "=", 2)
		if len(fields) == 2 {
			bk.buildargs[fields[0]] = fields[1]
		}
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

func buildkitBuild(ctx context.Context, env []string, platform, output string, opts buildkitOptions, buildlog io.Writer) error {
	reffile, err := os.CreateTemp("", "buildkit-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(reffile.Name())

	args := []string{
		"build",
		fmt.Sprintf("--frontend=%s", opts.frontend),
		"--local", fmt.Sprintf("context=%s", opts.context),
		"--local", fmt.Sprintf("dockerfile=%s", opts.context),
		"--opt", fmt.Sprintf("filename=%s", opts.filename),
		"--opt", fmt.Sprintf("platform=%s", platform),
	}
	if opts.target != "" {
		args = append(args,
			"--opt", fmt.Sprintf("target=%s", opts.target),
		)
	}
	for key, val := range opts.buildargs {
		args = append(args,
			"--opt", fmt.Sprintf("build-arg:%s=%s", key, val),
		)
	}
	args = append(args,
		"--output", fmt.Sprintf("type=oci,dest=%s", output),
		"--ref-file", reffile.Name(),
	)

	cmd := exec.CommandContext(ctx, "buildctl", args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	buildref, err := os.ReadFile(reffile.Name())
	if err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "buildctl", "debug", "logs", string(buildref))
	cmd.Stdout = buildlog
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func dockerBuild(ctx context.Context, env []string, platform, output string, opts buildkitOptions) error {
	args := []string{
		"build",
		"--quiet", // output id
		"--file", filepath.Join(opts.context, opts.filename),
		"--platform", platform,
		opts.context,
	}
	if opts.target != "" {
		args = append(args,
			"--target", opts.target,
		)
	}
	for key, val := range opts.buildargs {
		args = append(args,
			"--build-arg", fmt.Sprintf("%s=%s", key, val),
		)
	}

	buffer := bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = env
	cmd.Stdout = &buffer
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}
	id := strings.TrimSuffix(buffer.String(), "\n")

	cmd = exec.CommandContext(ctx, "docker", "save", "--output", output, id)
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (cp *BuildKitConveyorPacker) buildImage(ctx context.Context) error {
	tmpfile, err := os.CreateTemp("/var/tmp", "buildkit-*.tar")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	env := os.Environ()
	cfgdir, err := os.MkdirTemp("", "docker-config-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cfgdir)
	cfgfile := filepath.Join(cfgdir, "config.json")
	if cp.topts.AuthConfig != nil {
		cfg, err := cp.topts.AuthConfig.MarshalJSON()
		if err != nil {
			return err
		}
		err = os.WriteFile(cfgfile, cfg, 0o600)
		if err != nil {
			return err
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	err = os.Symlink(filepath.Join(home, ".docker", "cli-plugins"), filepath.Join(cfgdir, "cli-plugins"))
	if err != nil {
		return err
	}
	err = os.Symlink(filepath.Join(home, ".docker", "buildx"), filepath.Join(cfgdir, "buildx"))
	if err != nil {
		return err
	}
	env = append(env, fmt.Sprintf("DOCKER_CONFIG=%s", cfgdir))

	platform := cp.topts.Platform.String()

	output := tmpfile.Name()

	var imgCache *cache.Handle
	var ref string

	db := os.Getenv("DOCKER_BUILDKIT")
	if _, err := exec.LookPath("buildctl"); err != nil {
		// buildkit not found, use docker instead
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("neither %q nor %q could be found in the PATH, please install one of them to build", "buildctl", "docker")
		}
		db = "1"
	}

	if db == "" {
		if _, ok := os.LookupEnv("BUILDKIT_HOST"); !ok {
			sock := "/run/buildkit/buildkitd.sock"
			if _, err := os.Stat(sock); err != nil && errors.Is(err, os.ErrNotExist) {
				// make the error message from `buildctl` look more like the traditional error message from `docker`
				return fmt.Errorf("cannot connect to the BuildKit daemon at unix://%s. Is the buildkit daemon running?", sock)
			}
		}
		err = os.MkdirAll(filepath.Join(cp.b.RootfsPath, ".singularity.d"), 0o755)
		if err != nil {
			return err
		}
		buildlog, err := os.OpenFile(filepath.Join(cp.b.RootfsPath, "/.singularity.d/buildkit_build.log"), os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer buildlog.Close()

		err = buildkitBuild(ctx, env, platform, output, cp.bk, buildlog)
		if err != nil {
			return err
		}

		ref = "oci-archive:" + output
	} else {
		err := dockerBuild(ctx, env, platform, output, cp.bk)
		if err != nil {
			return err
		}

		ref = "docker-archive:" + output
	}

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
