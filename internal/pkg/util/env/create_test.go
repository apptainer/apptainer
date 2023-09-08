// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
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

//nolint:maintidx
func TestSetContainerEnv(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tt := []struct {
		name            string
		cleanEnv        bool
		homeDest        string
		env             []string
		processEnv      map[string]string
		resultEnv       []string
		apptainerEnv    map[string]string
		outputNeeded    []string
		outputNotNeeded []string
		disabled        bool
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
			outputNeeded: []string{
				"Can't process environment variable SOME_INVALID_VAR:test",
				"Not forwarding APPTAINER_CONTAINER environment variable",
				"Not forwarding APPTAINER_NAME environment variable",
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
			outputNeeded: []string{
				"Not forwarding APPTAINER_CONTAINER environment variable",
				"Not forwarding APPTAINER_NAME environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_APPEND_PATH as SING_USER_DEFINED_APPEND_PATH environment variable",
				"Forwarding APPTAINERENV_PATH as SING_USER_DEFINED_PATH environment variable",
				"APPTAINERENV_PREPEND_PATH as SING_USER_DEFINED_PREPEND_PATH environment variable",
				"Not forwarding APPTAINER_CONTAINER environment variable",
				"Not forwarding APPTAINER_NAME environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_FOO as FOO environment variable",
				"Not forwarding APPTAINER_CONTAINER environment variable",
				"Not forwarding APPTAINER_NAME environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_FOO as FOO environment variable",
				"Not forwarding APPTAINER_CONTAINER environment variable",
				"Not forwarding APPTAINER_NAME environment variable",
				"Forwarding TERM environment variable",
				"Forwarding http_proxy environment variable",
				"Forwarding https_proxy environment variable",
				"Forwarding no_proxy environment variable",
				"Forwarding all_proxy environment variable",
				"Forwarding ftp_proxy environment variable",
				"Forwarding HTTP_PROXY environment variable",
				"Forwarding HTTPS_PROXY environment variable",
				"Forwarding NO_PROXY environment variable",
				"Forwarding ALL_PROXY environment variable",
				"Forwarding FTP_PROXY environment variable",
				"Forwarding TERM environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_PATH as SING_USER_DEFINED_PATH environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_LANG as LANG environment variable",
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
			outputNeeded: []string{
				"Overriding HOME environment variable with APPTAINERENV_HOME is not permitted",
			},
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_LD_LIBRARY_PATH as LD_LIBRARY_PATH environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_LD_LIBRARY_PATH as LD_LIBRARY_PATH environment variable",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_HOST as HOST environment variable",
				"Environment variable HOST already has value [myhostenv], will not forward new value [myhost] from parent process environment",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_HOST as HOST environment variable",
				"Environment variable HOST already has value [myhostenv], will not forward new value [myhost] from parent process environment",
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_PRECEDENCE as PRECEDENCE environment variable",
				"Skipping environment variable [SINGULARITYENV_PRECEDENCE=second], PRECEDENCE is already overridden with different value [first]",
				"Environment variable PRECEDENCE already has value [first], will not forward new value [fifth] from parent process environment",
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
			outputNeeded: []string{
				"Environment variable SINGULARITYENV_PRECEDENCE is set, but APPTAINERENV_PRECEDENCE is preferred",
				"Forwarding SINGULARITYENV_PRECEDENCE as PRECEDENCE environment variable",
				"Environment variable PRECEDENCE already has value [second], will not forward new value [fifth] from parent process environment",
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
			outputNeeded: []string{},
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
			outputNeeded: []string{},
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_PRECEDENCE as PRECEDENCE environment variable",
				"Skipping environment variable [SINGULARITYENV_PRECEDENCE=precedence], PRECEDENCE is already overridden with the same value",
				"Environment variable PRECEDENCE already has duplicate value [precedence], will not forward from parent process environment",
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
			outputNeeded: []string{
				"Environment variable SINGULARITYENV_PRECEDENCE is set, but APPTAINERENV_PRECEDENCE is preferred",
				"Forwarding SINGULARITYENV_PRECEDENCE as PRECEDENCE environment variable",
				"Environment variable PRECEDENCE already has duplicate value [precedence], will not forward from parent process environment",
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
			outputNeeded: []string{
				"Forwarding PRECEDENCE environment variable",
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
			outputNeeded: []string{},
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
			outputNeeded: []string{
				"Forwarding APPTAINERENV_PRECEDENCE as PRECEDENCE environment variable",
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
			outputNeeded: []string{
				"Environment variable SINGULARITYENV_PRECEDENCE is set, but APPTAINERENV_PRECEDENCE is preferred",
				"Forwarding SINGULARITYENV_PRECEDENCE as PRECEDENCE environment variable",
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
			outputNeeded: []string{},
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
			outputNeeded: []string{},
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
			outputNeeded: []string{
				"Forwarding PRECEDENCE environment variable",
			},
		},
		{
			name:     "suppress the info message when both legacy and new env coexist",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"SINGULARITYENV_PS1=true",
				"APPTAINERENV_PS1=true",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			outputNotNeeded: []string{
				"Environment variable SINGULARITYENV_PS1 is set, but APPTAINERENV_PS1 is preferred",
			},
		},
		{
			name:     "should print info message if only legacy env exists",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"SINGULARITYENV_PS1=true",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			outputNeeded: []string{
				"Environment variable SINGULARITYENV_PS1 is set, but APPTAINERENV_PS1 is preferred",
			},
		},
		{
			name:     "should not print info message if only new env exists",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"APPTAINERENV_PS1=true",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			outputNotNeeded: []string{
				"Environment variable SINGULARITYENV_PS1 is set, but APPTAINERENV_PS1 is preferred",
			},
		},
		{
			name:     "should print warning message if legacy and new env vars have different values",
			cleanEnv: false,
			homeDest: "/home/tester",
			env: []string{
				"SINGULARITYENV_PS1=true",
				"APPTAINERENV_PS1=false",
			},
			resultEnv: []string{
				"HOME=/home/tester",
				"PATH=" + DefaultPath,
			},
			outputNeeded: []string{
				"SINGULARITYENV_PS1 and APPTAINERENV_PS1 have different values, using the latter",
			},
			outputNotNeeded: []string{
				"Environment variable SINGULARITYENV_PS1 is set, but APPTAINERENV_PS1 is preferred",
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
			for _, requiredOutput := range tc.outputNeeded {
				if !strings.Contains(output.String(), requiredOutput) {
					t.Errorf("Did not find required output: [%s]", requiredOutput)
				}
			}
			for _, notNeededOutput := range tc.outputNotNeeded {
				if strings.Contains(output.String(), notNeededOutput) {
					t.Errorf("[%s] should not exist in the output", notNeededOutput)
				}
			}
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
