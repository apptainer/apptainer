// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package env

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/apptainer/apptainer/pkg/sylog"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/internal/pkg/test"
)

func TestSetContainerEnv(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tt := []struct {
		name         string
		cleanEnv     bool
		homeDest     string
		env          []string
		processEnv   map[string]string
		resultEnv    []string
		apptainerEnv map[string]string
		disabled     bool
	}{
		{
			name:     "no APPTAINERENV_",
			homeDest: "/home/tester",
			env: []string{
				"LD_LIBRARY_PATH=/.singularity.d/libs",
				"HOME=/home/john",
				"SOME_INVALID_VAR:test",
				"APPTAINERENV_=invalid",
				"PS1=test",
				"TERM=xterm-256color",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"PWD=/tmp",
				"LC_ALL=C",
				"APPTAINER_NAME=lolcow.sif",
			},
			resultEnv: []string{
				"PS1=test",
				"TERM=xterm-256color",
				"LANG=C",
				"PWD=/tmp",
				"LC_ALL=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
		},
		{
			name:     "exclude PATH",
			homeDest: "/home/tester",
			env: []string{
				"LD_LIBRARY_PATH=/.singularity.d/libs",
				"HOME=/home/john",
				"PS1=test",
				"SOCIOPATH=VolanDeMort",
				"TERM=xterm-256color",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINERENV_LD_LIBRARY_PATH=/my/custom/libs",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"PWD=/tmp",
				"LC_ALL=C",
				"APPTAINER_NAME=lolcow.sif",
			},
			resultEnv: []string{
				"PS1=test",
				"SOCIOPATH=VolanDeMort",
				"TERM=xterm-256color",
				"LANG=C",
				"PWD=/tmp",
				"LC_ALL=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"LD_LIBRARY_PATH": "/my/custom/libs",
			},
		},
		{
			name:     "special PATH envs",
			homeDest: "/home/tester",
			env: []string{
				"LD_LIBRARY_PATH=/.singularity.d/libs",
				"HOME=/home/john",
				"APPTAINERENV_APPEND_PATH=/sylabs/container",
				"PS1=test",
				"TERM=xterm-256color",
				"APPTAINERENV_PATH=/my/path",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"PWD=/tmp",
				"LC_ALL=C",
				"APPTAINERENV_PREPEND_PATH=/foo/bar",
				"APPTAINER_NAME=lolcow.sif",
			},
			resultEnv: []string{
				"PS1=test",
				"TERM=xterm-256color",
				"LANG=C",
				"PWD=/tmp",
				"LC_ALL=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"SING_USER_DEFINED_PREPEND_PATH": "/foo/bar",
				"SING_USER_DEFINED_PATH":         "/my/path",
				"SING_USER_DEFINED_APPEND_PATH":  "/sylabs/container",
			},
		},
		{
			name:     "clean envs",
			cleanEnv: true,
			homeDest: "/home/tester",
			env: []string{
				"LD_LIBRARY_PATH=/.singularity.d/libs",
				"HOME=/home/john",
				"PS1=test",
				"TERM=xterm-256color",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"PWD=/tmp",
				"LC_ALL=C",
				"APPTAINER_NAME=lolcow.sif",
				"APPTAINERENV_FOO=VAR",
				"CLEANENV=TRUE",
			},
			resultEnv: []string{
				"LANG=C",
				"TERM=xterm-256color",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"FOO": "VAR",
			},
		},
		{
			name:     "always pass keys",
			cleanEnv: true,
			homeDest: "/home/tester",
			env: []string{
				"LD_LIBRARY_PATH=/.singularity.d/libs",
				"HOME=/home/john",
				"PS1=test",
				"TERM=xterm-256color",
				"PATH=/usr/games:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=C",
				"APPTAINER_CONTAINER=/tmp/lolcow.sif",
				"PWD=/tmp",
				"LC_ALL=C",
				"http_proxy=http_proxy",
				"https_proxy=https_proxy",
				"no_proxy=no_proxy",
				"all_proxy=all_proxy",
				"ftp_proxy=ftp_proxy",
				"HTTP_PROXY=http_proxy",
				"HTTPS_PROXY=https_proxy",
				"NO_PROXY=no_proxy",
				"ALL_PROXY=all_proxy",
				"FTP_PROXY=ftp_proxy",
				"APPTAINER_NAME=lolcow.sif",
				"APPTAINERENV_FOO=VAR",
				"CLEANENV=TRUE",
			},
			resultEnv: []string{
				"LANG=C",
				"TERM=xterm-256color",
				"http_proxy=http_proxy",
				"https_proxy=https_proxy",
				"no_proxy=no_proxy",
				"all_proxy=all_proxy",
				"ftp_proxy=ftp_proxy",
				"HTTP_PROXY=http_proxy",
				"HTTPS_PROXY=https_proxy",
				"NO_PROXY=no_proxy",
				"ALL_PROXY=all_proxy",
				"FTP_PROXY=ftp_proxy",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"FOO": "VAR",
			},
		},
		{
			name:     "APPTAINERENV_PATH",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_PATH=/my/path",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"SING_USER_DEFINED_PATH": "/my/path",
			},
		},
		{
			name:     "APPTAINERENV_LANG with cleanenv",
			cleanEnv: true,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_LANG=en",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"LANG": "en",
			},
		},
		{
			name:     "APPTAINERENV_HOME",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_HOME=/my/home",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{},
		},
		{
			name:     "APPTAINERENV_LD_LIBRARY_PATH",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_LD_LIBRARY_PATH=/my/libs",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"LD_LIBRARY_PATH": "/my/libs",
			},
		},
		{
			name:     "APPTAINERENV_LD_LIBRARY_PATH with cleanenv",
			cleanEnv: true,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_LD_LIBRARY_PATH=/my/libs",
			},
			resultEnv: []string{
				"LANG=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"LD_LIBRARY_PATH": "/my/libs",
			},
		},
		{
			name:     "APPTAINERENV_HOST after HOST",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"HOST=myhost",
				"APPTAINERENV_HOST=myhostenv",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"HOST": "myhostenv",
			},
		},
		{
			name:     "APPTAINERENV_HOST before HOST",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_HOST=myhostenv",
				"HOST=myhost",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"HOST": "myhostenv",
			},
		},
		// test permutations of named environment variable with
		// differing and no prefix -- confirm precedence

		{
			name:     "ENV precedence - first - with conflicts - different values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_PRECEDENCE=first",
				"SINGULARITYENV_PRECEDENCE=second",
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=fifth",
			},
			processEnv: map[string]string{
				"PRECEDENCE": "third",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"PRECEDENCE": "first",
			},
		},
		{
			name:     "ENV precedence - second - with conflicts - different values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"SINGULARITYENV_PRECEDENCE=second",
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=fifth",
			},
			processEnv: map[string]string{
				"PRECEDENCE": "third",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"PRECEDENCE": "second",
			},
		},
		{
			disabled: true,
			name:     "ENV precedence - third - with conflicts - different values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=fifth",
			},
			processEnv: map[string]string{
				"PRECEDENCE": "third",
			},
			resultEnv: []string{
				"PRECEDENCE=third",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
		},
		{
			name:     "ENV precedence - fourth - with conflicts - different values",
			cleanEnv: true,
			homeDest: "/home/tester",
			env: []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=fifth",
			},
			resultEnv: []string{
				"LANG=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{},
		},
		{
			name:     "ENV precedence - first - with conflicts - same values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_PRECEDENCE=precedence",
				"SINGULARITYENV_PRECEDENCE=precedence",
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=precedence",
			},
			processEnv: map[string]string{
				"PRECEDENCE": "precedence",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"PRECEDENCE": "precedence",
			},
		},
		{
			name:     "ENV precedence - second - with conflicts - same values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"SINGULARITYENV_PRECEDENCE=precedence",
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=precedence",
			},
			processEnv: map[string]string{
				"PRECEDENCE": "precedence",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"PRECEDENCE": "precedence",
			},
		},
		{
			name:     "ENV precedence - third - with conflicts - same values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=precedence",
			},
			processEnv: map[string]string{
				"PRECEDENCE": "precedence",
			},
			resultEnv: []string{
				"PRECEDENCE=precedence",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
		},
		{
			name:     "ENV precedence - fourth - with conflicts - same values",
			cleanEnv: true,
			homeDest: "/home/tester",
			env: []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=precedence",
			},
			resultEnv: []string{
				"LANG=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{},
		},
		{
			name:     "ENV precedence - first - with no conflicts",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_PRECEDENCE=first",
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"PRECEDENCE": "first",
			},
		},
		{
			name:     "ENV precedence - second - with no conflicts",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"SINGULARITYENV_PRECEDENCE=second",
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{
				"PRECEDENCE": "second",
			},
		},
		{
			name:     "ENV precedence - third - with no conflicts",
			cleanEnv: false,
			homeDest: "/home/tester",
			env:      []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
			},
			processEnv: map[string]string{
				"PRECEDENCE": "third",
			},
			resultEnv: []string{
				"PRECEDENCE=third",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{},
		},
		{
			name:     "ENV precedence - fourth - with no conflicts",
			cleanEnv: true,
			homeDest: "/home/tester",
			env:      []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
			},
			resultEnv: []string{
				"LANG=C",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			apptainerEnv: map[string]string{},
		},
		{
			name:     "ENV precedence - fifth/last - with no conflicts",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				// third precedence is environment variables initialized via --env, or --env-file
				// fourth precedence is result of "clean env" flag option
				"PRECEDENCE=fifth",
			},
			resultEnv: []string{
				"PRECEDENCE=fifth",
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
		},
	}
	for _, tc := range tt {
		if tc.disabled {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			ociConfig := &oci.Config{}
			generator := generate.New(&ociConfig.Spec)
			if nil != tc.processEnv {
				// add vars for --env or --env-file
				generator.Config.Process = &specs.Process{}
				for k, v := range tc.processEnv {
					generator.SetProcessEnv(k, v)
				}
			}
			output := bytes.Buffer{}
			var senv map[string]string
			func() {
				oldWriter := sylog.SetWriter(&output)
				oldLevel := sylog.GetLevel()
				sylog.SetLevel(int(sylog.DebugLevel), true)
				defer func() {
					oldWriter.Write(output.Bytes())
					sylog.SetWriter(oldWriter)
					sylog.SetLevel(oldLevel, true)
				}()
				senv = SetContainerEnv(generator, tc.env, tc.cleanEnv, tc.homeDest)
			}()
			if !equal(t, ociConfig.Process.Env, tc.resultEnv) {
				t.Fatalf("unexpected envs:\n want: %v\ngot: %v", tc.resultEnv, ociConfig.Process.Env)
			}
			if tc.apptainerEnv != nil && !reflect.DeepEqual(senv, tc.apptainerEnv) {
				t.Fatalf("unexpected apptainer env:\n want: %v\ngot: %v", tc.apptainerEnv, senv)
			}
		})
	}
}

// equal tells whether a and b contain the same elements in the
// same order. A nil argument is equivalent to an empty slice.
func equal(t *testing.T, a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if c := strings.Compare(v, b[i]); c != 0 {
			return false
		}
	}
	return true
}
