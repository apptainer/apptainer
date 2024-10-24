// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/internal/pkg/ociplatform"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/internal/pkg/util/shell"
	sytypes "github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type ociRunscriptData struct {
	PrependCmd        string
	PrependEntrypoint string
}

//nolint:dupword
const ociRunscript = `
# When SINGULARITY_NO_EVAL set, use OCI compatible behavior that does
# not evaluate resolved CMD / ENTRYPOINT / ARGS through the shell, and
# does not modify expected quoting behavior of args.
if [ -n "$SINGULARITY_NO_EVAL" ]; then
    # ENTRYPOINT only - run entrypoint plus args
    if [ -z "$OCI_CMD" ] && [ -n "$OCI_ENTRYPOINT" ]; then
        {{ .PrependEntrypoint }}
        exec "$@"
    fi

    # CMD only - run CMD or override with args
    if [ -n "$OCI_CMD" ] && [ -z "$OCI_ENTRYPOINT" ]; then
        {{- if .PrependCmd }}
        if [ $# -eq 0 ]; then
            {{ .PrependCmd }}
        fi
        {{- end }}
        exec "$@"
    fi

    # ENTRYPOINT and CMD - run ENTRYPOINT with CMD as default args
    # override with user provided args
    {{- if .PrependEntrypoint }}
    if [ $# -gt 0 ]; then
        {{ .PrependEntrypoint }}
	else
        {{ .PrependCmd }}
        {{ .PrependEntrypoint }}
    fi
	{{- else if .PrependCmd }}
    if [ $# -eq 0 ]; then
        {{ .PrependCmd }}
    fi
    {{- end }}
    exec "$@"
fi

# Standard Apptainer behavior evaluates CMD / ENTRYPOINT / ARGS
# combination through shell before exec, and requires special quoting
# due to concatenation of CMDLINE_ARGS.
CMDLINE_ARGS=""
# prepare command line arguments for evaluation
for arg in "$@"; do
        CMDLINE_ARGS="${CMDLINE_ARGS} \"$arg\""
done

# ENTRYPOINT only - run entrypoint plus args
if [ -z "$OCI_CMD" ] && [ -n "$OCI_ENTRYPOINT" ]; then
    if [ $# -gt 0 ]; then
        SINGULARITY_OCI_RUN="${OCI_ENTRYPOINT} ${CMDLINE_ARGS}"
    else
        SINGULARITY_OCI_RUN="${OCI_ENTRYPOINT}"
    fi
fi

# CMD only - run CMD or override with args
if [ -n "$OCI_CMD" ] && [ -z "$OCI_ENTRYPOINT" ]; then
    if [ $# -gt 0 ]; then
        SINGULARITY_OCI_RUN="${CMDLINE_ARGS}"
    else
        SINGULARITY_OCI_RUN="${OCI_CMD}"
    fi
fi

# ENTRYPOINT and CMD - run ENTRYPOINT with CMD as default args
# override with user provided args
if [ $# -gt 0 ]; then
    SINGULARITY_OCI_RUN="${OCI_ENTRYPOINT} ${CMDLINE_ARGS}"
else
    SINGULARITY_OCI_RUN="${OCI_ENTRYPOINT} ${OCI_CMD}"
fi

# Evaluate shell expressions first and set arguments accordingly,
# then execute final command as first container process
eval "set ${SINGULARITY_OCI_RUN}"
exec "$@"
`

const variableRegex = `^[a-zA-Z_]{1,}[a-zA-Z0-9_]{0,}$`

// OCIConveyorPacker holds stuff that needs to be packed into the bundle
type OCIConveyorPacker struct {
	srcImg    v1.Image
	b         *sytypes.Bundle
	imgConfig v1.Config
	topts     *ociimage.TransportOptions
}

// Get downloads container information from the specified source
func (cp *OCIConveyorPacker) Get(ctx context.Context, b *sytypes.Bundle) (err error) {
	sylog.Infof("Fetching OCI image...")
	cp.b = b

	cp.topts = &ociimage.TransportOptions{
		Insecure:         cp.b.Opts.NoHTTPS,
		DockerDaemonHost: cp.b.Opts.DockerDaemonHost,
		AuthConfig:       cp.b.Opts.OCIAuthConfig,
		AuthFilePath:     ociauth.ChooseAuthFile(cp.b.Opts.ReqAuthFile),
		UserAgent:        useragent.Value(),
		TmpDir:           b.TmpDir,
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

	// Add registry and namespace to image reference if specified
	ref := b.Recipe.Header["from"]
	if b.Recipe.Header["namespace"] != "" {
		ref = b.Recipe.Header["namespace"] + "/" + ref
	}
	if b.Recipe.Header["registry"] != "" {
		ref = b.Recipe.Header["registry"] + "/" + ref
	}
	// Docker sources are docker://<from>, not docker:<from>
	if b.Recipe.Header["bootstrap"] == "docker" {
		ref = "//" + ref
	}
	// Prefix bootstrap type to image reference
	ref = b.Recipe.Header["bootstrap"] + ":" + ref

	var imgCache *cache.Handle
	if !cp.b.Opts.NoCache {
		imgCache = cp.b.Opts.ImgCache
	}

	// Fetch the image into a temporary containers/image oci layout dir.
	cp.srcImg, err = ociimage.FetchToLayout(ctx, cp.topts, imgCache, ref, b.TmpDir)
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

// Pack puts relevant objects in a Bundle.
func (cp *OCIConveyorPacker) Pack(ctx context.Context) (*sytypes.Bundle, error) {
	sylog.Infof("Extracting OCI image...")
	err := cp.unpackRootfs(ctx)
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

func (cp *OCIConveyorPacker) insertOCIConfig() error {
	conf, err := json.Marshal(cp.imgConfig)
	if err != nil {
		return err
	}

	cp.b.JSONObjects[image.SIFDescOCIConfigJSON] = conf
	return nil
}

func (cp *OCIConveyorPacker) unpackRootfs(ctx context.Context) error {
	if err := UnpackRootfs(ctx, cp.srcImg, cp.b.RootfsPath); err != nil {
		return err
	}

	// If the `--fix-perms` flag was used, then modify the permissions so that
	// content has owner rwX and we're done
	if cp.b.Opts.FixPerms {
		sylog.Warningf("The --fix-perms option modifies the filesystem permissions on the resulting container.")
		sylog.Debugf("Modifying permissions for file/directory owners")
		return FixPerms(cp.b.RootfsPath)
	}

	// If `--fix-perms` was not used and this is a sandbox, scan for restrictive
	// perms that would stop the user doing an `rm` without a chmod first,
	// and warn if they exist
	if cp.b.Opts.SandboxTarget {
		sylog.Debugf("Scanning for restrictive permissions")
		return CheckPerms(cp.b.RootfsPath)
	}

	return nil
}

func (cp *OCIConveyorPacker) insertBaseEnv() (err error) {
	if err = makeBaseEnv(cp.b.RootfsPath); err != nil {
		sylog.Errorf("%v", err)
	}
	return
}

func (cp *OCIConveyorPacker) insertRunScript() error {
	f, err := os.Create(cp.b.RootfsPath + "/.singularity.d/runscript")
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString("#!/bin/sh\n")
	if err != nil {
		return err
	}

	if len(cp.imgConfig.Entrypoint) > 0 {
		_, err = f.WriteString("OCI_ENTRYPOINT='" +
			shell.EscapeSingleQuotes(shell.ArgsQuoted(cp.imgConfig.Entrypoint)) +
			"'\n")
		if err != nil {
			return err
		}
	} else {
		_, err = f.WriteString("OCI_ENTRYPOINT=''\n")
		if err != nil {
			return err
		}
	}

	if len(cp.imgConfig.Cmd) > 0 {
		_, err = f.WriteString("OCI_CMD='" +
			shell.EscapeSingleQuotes(shell.ArgsQuoted(cp.imgConfig.Cmd)) +
			"'\n")
		if err != nil {
			return err
		}
	} else {
		_, err = f.WriteString("OCI_CMD=''\n")
		if err != nil {
			return err
		}
	}

	// prependCmd is a set of shell commands necessary to prepend each CMD entry to $@
	prependCmd := ""
	for i := len(cp.imgConfig.Cmd) - 1; i >= 0; i-- {
		prependCmd = prependCmd + fmt.Sprintf("set -- '%s' \"$@\"\n", shell.EscapeSingleQuotes(cp.imgConfig.Cmd[i]))
	}
	// prependCmd is a set of shell commands necessary to prepend each ENTRYPOINT entry to $@
	prependEP := ""
	for i := len(cp.imgConfig.Entrypoint) - 1; i >= 0; i-- {
		prependEP = prependEP + fmt.Sprintf("set -- '%s' \"$@\"\n", shell.EscapeSingleQuotes(cp.imgConfig.Entrypoint[i]))
	}

	data := ociRunscriptData{
		PrependCmd:        prependCmd,
		PrependEntrypoint: prependEP,
	}

	tmpl, err := template.New("runscript").Parse(ociRunscript)
	if err != nil {
		return fmt.Errorf("while parsing runscript template: %w", err)
	}

	var runscript bytes.Buffer
	err = tmpl.Execute(&runscript, data)
	if err != nil {
		return fmt.Errorf("while generating runscript template: %w", err)
	}

	_, err = f.WriteString(runscript.String())
	if err != nil {
		return err
	}

	f.Sync()

	err = os.Chmod(cp.b.RootfsPath+"/.singularity.d/runscript", 0o755)
	if err != nil {
		return err
	}

	return nil
}

func (cp *OCIConveyorPacker) insertEnv() error {
	f, err := os.Create(cp.b.RootfsPath + "/.singularity.d/env/10-docker2singularity.sh")
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString("#!/bin/sh\n")
	if err != nil {
		return err
	}

	varRegex := regexp.MustCompile(variableRegex)
	for _, element := range cp.imgConfig.Env {
		export := ""
		envParts := strings.SplitN(element, "=", 2)
		if matched := varRegex.MatchString(envParts[0]); !matched {
			sylog.Verbosef("env %s has invalid format, skip insertion", envParts[0])
			continue
		}
		if len(envParts) == 1 {
			export = fmt.Sprintf("export %s=\"${%s:-}\"\n", envParts[0], envParts[0])
		} else {
			if envParts[0] == "PATH" {
				export = fmt.Sprintf("export %s=%q\n", envParts[0], shell.Escape(envParts[1]))
			} else {
				export = fmt.Sprintf("export %s=\"${%s:-%q}\"\n", envParts[0], envParts[0], shell.Escape(envParts[1]))
			}
		}
		_, err = f.WriteString(export)
		if err != nil {
			return err
		}
	}

	f.Sync()

	err = os.Chmod(cp.b.RootfsPath+"/.singularity.d/env/10-docker2singularity.sh", 0o755)
	if err != nil {
		return err
	}

	return nil
}

func (cp *OCIConveyorPacker) insertOCILabels() (err error) {
	labels := cp.imgConfig.Labels
	var text []byte

	// make new map into json
	text, err = json.MarshalIndent(labels, "", "\t")
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(cp.b.RootfsPath, "/.singularity.d/labels.json"), []byte(text), 0o644)
	return err
}

// CleanUp removes any tmpfs owned by the conveyorPacker on the filesystem
func (cp *OCIConveyorPacker) CleanUp() {
	cp.b.Remove()
}
