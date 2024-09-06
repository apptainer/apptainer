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
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/apptainer/apptainer/internal/pkg/build/oci"
	"github.com/apptainer/apptainer/internal/pkg/util/ociauth"
	"github.com/apptainer/apptainer/internal/pkg/util/shell"
	sytypes "github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	dockerdaemon "github.com/containers/image/v5/docker/daemon"
	ociarchive "github.com/containers/image/v5/oci/archive"
	ocilayout "github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
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

// OCIConveyorPacker holds stuff that needs to be packed into the bundle
type OCIConveyorPacker struct {
	srcRef    types.ImageReference
	b         *sytypes.Bundle
	tmpfsRef  types.ImageReference
	policyCtx *signature.PolicyContext
	imgConfig imgspecv1.ImageConfig
	sysCtx    *types.SystemContext
}

// Get downloads container information from the specified source
func (cp *OCIConveyorPacker) Get(ctx context.Context, b *sytypes.Bundle) (err error) {
	cp.b = b

	policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	cp.policyCtx, err = signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}

	// DockerInsecureSkipTLSVerify is set only if --no-https is specified to honor
	// configuration from /etc/containers/registries.conf because DockerInsecureSkipTLSVerify
	// can have three possible values true/false and undefined, so we left it as undefined instead
	// of forcing it to false in order to delegate decision to /etc/containers/registries.conf:
	// https://github.com/apptainer/singularity/issues/5172

	cp.sysCtx = &types.SystemContext{
		OCIInsecureSkipTLSVerify: cp.b.Opts.NoHTTPS,
		DockerAuthConfig:         cp.b.Opts.DockerAuthConfig,
		DockerDaemonHost:         cp.b.Opts.DockerDaemonHost,
		OSChoice:                 "linux",
		AuthFilePath:             ociauth.ChooseAuthFile(cp.b.Opts.ReqAuthFile),
		DockerRegistryUserAgent:  useragent.Value(),
		BigFilesTemporaryDir:     b.TmpDir,
	}
	if cp.b.Opts.Arch != "" {
		if arch, ok := oci.ArchMap[cp.b.Opts.Arch]; ok {
			cp.sysCtx.ArchitectureChoice = arch.Arch
			cp.sysCtx.VariantChoice = arch.Var
		} else {
			keys := reflect.ValueOf(oci.ArchMap).MapKeys()
			return fmt.Errorf("failed to parse the arch value: %s, should be one of %v", cp.b.Opts.Arch, keys)
		}
	}

	if cp.b.Opts.NoHTTPS {
		cp.sysCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}

	// add registry and namespace to reference if specified
	ref := b.Recipe.Header["from"]
	if b.Recipe.Header["namespace"] != "" {
		ref = b.Recipe.Header["namespace"] + "/" + ref
	}
	if b.Recipe.Header["registry"] != "" {
		ref = b.Recipe.Header["registry"] + "/" + ref
	}
	sylog.Debugf("Reference: %v", ref)

	switch b.Recipe.Header["bootstrap"] {
	case "docker":
		ref = "//" + ref
		cp.srcRef, err = docker.ParseReference(ref)
	case "docker-archive":
		cp.srcRef, err = dockerarchive.ParseReference(ref)
	case "docker-daemon":
		cp.srcRef, err = dockerdaemon.ParseReference(ref)
	case "oci":
		cp.srcRef, err = ocilayout.ParseReference(ref)
	case "oci-archive":
		if os.Geteuid() == 0 {
			// As root, the direct oci-archive handling will work
			cp.srcRef, err = ociarchive.ParseReference(ref)
		} else {
			// As non-root we need to do a dumb tar extraction first
			tmpDir, err := os.MkdirTemp(b.TmpDir, "temp-oci-")
			if err != nil {
				return fmt.Errorf("could not create temporary oci directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			refParts := strings.SplitN(b.Recipe.Header["from"], ":", 2)
			err = cp.extractArchive(refParts[0], tmpDir)
			if err != nil {
				return fmt.Errorf("error extracting the OCI archive file: %v", err)
			}
			// We may or may not have had a ':tag' in the source to handle
			if len(refParts) == 2 {
				cp.srcRef, err = ocilayout.ParseReference(tmpDir + ":" + refParts[1])
			} else {
				cp.srcRef, err = ocilayout.ParseReference(tmpDir)
			}

			if err != nil {
				return fmt.Errorf("error parsing reference: %v", err)
			}
		}

	default:
		return fmt.Errorf("oci conveyorPacker does not support %s", b.Recipe.Header["bootstrap"])
	}

	if err != nil {
		return fmt.Errorf("invalid image source: %v", err)
	}

	if !cp.b.Opts.NoCache {
		// Grab the modified source ref from the cache
		cp.srcRef, err = oci.ConvertReference(ctx, b.Opts.ImgCache, cp.srcRef, cp.sysCtx)
		if err != nil {
			return fmt.Errorf("while converting reference: %w", err)
		}
	}

	// To to do the RootFS extraction we also have to have a location that
	// contains *only* this image
	cp.tmpfsRef, err = ocilayout.ParseReference(cp.b.TmpDir + ":" + "tmp")
	if err != nil {
		return fmt.Errorf("while parsing reference: %w", err)
	}

	err = cp.fetch(ctx)
	if err != nil {
		return fmt.Errorf("while fetching image: %w", err)
	}

	cp.imgConfig, err = cp.getConfig(ctx)
	if err != nil {
		return fmt.Errorf("while getting config: %w", err)
	}

	return nil
}

// Pack puts relevant objects in a Bundle.
func (cp *OCIConveyorPacker) Pack(ctx context.Context) (*sytypes.Bundle, error) {
	err := cp.unpackTmpfs(ctx)
	if err != nil {
		return nil, fmt.Errorf("while unpacking tmpfs: %v", err)
	}

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

func (cp *OCIConveyorPacker) fetch(ctx context.Context) error {
	// cp.srcRef contains the cache source reference
	_, err := copy.Image(ctx, cp.policyCtx, cp.tmpfsRef, cp.srcRef, &copy.Options{
		ReportWriter:     io.Discard,
		SourceCtx:        cp.sysCtx,
		RemoveSignatures: true,
	})
	return err
}

func (cp *OCIConveyorPacker) getConfig(ctx context.Context) (imgspecv1.ImageConfig, error) {
	img, err := cp.srcRef.NewImage(ctx, cp.sysCtx)
	if err != nil {
		return imgspecv1.ImageConfig{}, err
	}
	defer img.Close()

	imgSpec, err := img.OCIConfig(ctx)
	if err != nil {
		return imgspecv1.ImageConfig{}, err
	}
	return imgSpec.Config, nil
}

func (cp *OCIConveyorPacker) insertOCIConfig() error {
	conf, err := json.Marshal(cp.imgConfig)
	if err != nil {
		return err
	}

	cp.b.JSONObjects[image.SIFDescOCIConfigJSON] = conf
	return nil
}

// Perform a dumb tar(gz) extraction with no chown, id remapping etc.
// This is needed for non-root handling of `oci-archive` as the extraction
// by containers/archive is failing when uid/gid don't match local machine
// and we're not root
func (cp *OCIConveyorPacker) extractArchive(src string, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	header, err := r.Peek(10) // read a few bytes without consuming
	if err != nil {
		return err
	}
	gzipped := strings.Contains(http.DetectContentType(header), "x-gzip")

	if gzipped {
		r, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer r.Close()
	}

	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// ZipSlip protection - don't escape from dst
		// #nosec G305
		target := filepath.Join(dst, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal extraction path", target)
		}

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			}
		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec
				return err
			}
		}
	}
}

func (cp *OCIConveyorPacker) unpackTmpfs(ctx context.Context) error {
	return unpackRootfs(ctx, cp.b, cp.tmpfsRef, cp.sysCtx)
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

	for _, element := range cp.imgConfig.Env {
		export := ""
		envParts := strings.SplitN(element, "=", 2)
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
