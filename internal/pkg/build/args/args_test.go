// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package args

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/apptainer/apptainer/pkg/build/types/parser"
)

func TestGetKeyVal(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
		err    string
	}{
		{
			name:  "normal case",
			input: "  k1= v1\n",
			expect: []string{
				"k1", "v1",
			},
			err: "",
		},
		{
			name:  "normal case 2",
			input: "\n  k1 = v1\n",
			expect: []string{
				"k1", "v1",
			},
			err: "",
		},
		{
			name:  "normal case 3",
			input: "k1=1.0",
			expect: []string{
				"k1", "1.0",
			},
			err: "",
		},
		{
			name:  "normal case 4",
			input: "k1= a whitespace  ",
			expect: []string{
				"k1", "a whitespace",
			},
			err: "",
		},
		{
			name:   "wrong case because of missing =",
			input:  "\n  k1 v1\n",
			expect: []string{},
			err:    "is not a key=value pair",
		},
		{
			name:   "wrong case because of missing key",
			input:  "\n  =v1\n",
			expect: []string{},
			err:    "missing key portion in",
		},
		{
			name:   "ok case with empty value",
			input:  "\n  k1 =\n",
			expect: []string{"k1", ""},
			err:    "",
		},
		{
			name:   "ok case empty value with multiple space",
			input:  "\n  k1 =  \n",
			expect: []string{"k1", ""},
			err:    "",
		},
		{
			name:   "ok case single quote",
			input:  "\n  k1 =''\n",
			expect: []string{"k1", "''"},
			err:    "",
		},
		{
			name:   "ok case single quote with multiple space",
			input:  "\n  k1 ='  '\n",
			expect: []string{"k1", "'  '"},
			err:    "",
		},
		{
			name:   "ok case double quote",
			input:  "\n  k1 =\"\"\n",
			expect: []string{"k1", "\"\""},
			err:    "",
		},
		{
			name:   "ok case double quote with multiple space",
			input:  "\n  k1 =\"   \"\n",
			expect: []string{"k1", "\"   \""},
			err:    "",
		},
	}

	for _, test := range tests {
		t.Logf("Starting %s", test.name)
		k, v, err := getKeyVal(test.input)
		if test.err != "" {
			assert.ErrorContains(t, err, test.err)
		} else {
			assert.Equal(t, k, test.expect[0])
			assert.Equal(t, v, test.expect[1])
		}
	}
}

func TestReader(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		output         string
		argsMap        map[string]string
		defaultArgsMap map[string]string
		err            string
	}{
		{
			name:   "normal case",
			input:  "/script-{{ APP_VER }}",
			output: "/script-1.0",
			argsMap: map[string]string{
				"OS_VER":  "1",
				"APP_VER": "1.0",
			},
			defaultArgsMap: map[string]string{},
			err:            "",
		},
		{
			name:   "normal case 2",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "/script-1.sh 1.0",
			argsMap: map[string]string{
				"OS_VER":  "1",
				"APP_VER": "1.0",
			},
			defaultArgsMap: map[string]string{},
			err:            "",
		},
		{
			name:           "normal case 3",
			input:          "/script-1.sh 1.0",
			output:         "/script-1.sh 1.0",
			argsMap:        map[string]string{},
			defaultArgsMap: map[string]string{},
			err:            "",
		},
		{
			name:   "normal case 4",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "/script-1.sh 1.0",
			argsMap: map[string]string{
				"OS_VER": "1",
			},
			defaultArgsMap: map[string]string{
				"APP_VER": "1.0",
			},
			err: "",
		},
		{
			name:   "normal case 5",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "/script-1.sh 1.0",
			argsMap: map[string]string{
				"OS_VER":  "1",
				"APP_VER": "1.0",
			},
			defaultArgsMap: map[string]string{
				"APP_VER": "0.0",
			},
			err: "",
		},
		{
			name:   "normal case 6",
			input:  "/script-{{ \nAPP_VER        }}",
			output: "/script-1.0",
			argsMap: map[string]string{
				"OS_VER":  "1",
				"APP_VER": "1.0",
			},
			defaultArgsMap: map[string]string{},
			err:            "",
		},
		{
			name:   "wrong case because of missing variable",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "",
			argsMap: map[string]string{
				"OS_VER": "1",
			},
			defaultArgsMap: map[string]string{},
			err:            "is not defined through either --build-arg (--build-arg-file) or 'arguments' section",
		},
		{
			name:   "wrong case because of missing variable 2",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "",
			argsMap: map[string]string{
				"OS_VE": "1",
			},
			defaultArgsMap: map[string]string{},
			err:            "is not defined through either --build-arg (--build-arg-file) or 'arguments' section",
		},
		{
			name: "ok case with variables defined in comment lines",
			input: `
			%arguments
				OS_VER=1 #  comment line {{ OS_VER }}
			%post
			    	# comment
				#an
				#!/bin/{{ BASH }}
				apt install {{ OS_VER }}#comment {{ OS_VER }}
				#some other comment {{ OS_VER }}
				#should not be replaced as well 
			`,
			output: `
			%arguments
				OS_VER=1 #  comment line {{ OS_VER }}
			%post
			    	# comment
				#an
				#!/bin/csh
				apt install 1#comment {{ OS_VER }}
				#some other comment {{ OS_VER }}
				#should not be replaced as well 
			`,
			argsMap: map[string]string{
				"OS_VER": "1",
				"BASH":   "csh",
			},
			defaultArgsMap: map[string]string{},
			err:            "",
		},
	}

	for _, test := range tests {
		t.Logf("Starting %s", test.name)
		var consumedArgs []string
		reader, err := NewReader(
			bytes.NewReader([]byte(test.input)),
			test.argsMap,
			test.defaultArgsMap,
			&consumedArgs,
		)

		var output []byte
		if err == nil {
			output, err = io.ReadAll(reader)
		}

		if test.err != "" {
			assert.ErrorContains(t, err, test.err)
		} else {
			assert.Equal(t, string(output), test.output)
		}
	}
}

func TestReadDefaults(t *testing.T) {
	defFilePath := filepath.Join("..", "..", "..", "..", "test", "build-args", "single-stage-unit-test.def")
	defFile, err := os.Open(defFilePath)
	if err != nil {
		t.Fatalf("while trying to open def file %q: %s", defFilePath, err)
	}
	defer defFile.Close()
	defs, err := parser.All(defFile)
	if err != nil {
		t.Fatalf("while trying to read def file %q: %s", defFilePath, err)
	}
	assert.Equal(t, len(defs), 1)
	defaultArgsMap := ReadDefaults(defs[0])
	assert.DeepEqual(t, defaultArgsMap, map[string]string{
		"OS_VER": "3.17",
		"DEMO":   "a demo",
		"AUTHOR": "jason",
	})

	defFilePath = filepath.Join("..", "..", "..", "..", "test", "build-args", "multiple-stage-unit-test.def")
	defFile, err = os.Open(defFilePath)
	if err != nil {
		t.Fatalf("while trying to open def file %q: %s", defFilePath, err)
	}
	defer defFile.Close()
	defs, err = parser.All(defFile)
	if err != nil {
		t.Fatalf("while trying to read def file %q: %s", defFilePath, err)
	}
	assert.Equal(t, len(defs), 2)
	defaultArgsMap = ReadDefaults(defs[0])
	assert.DeepEqual(t, defaultArgsMap, map[string]string{
		"HOME": "/root",
	})
	defaultArgsMap = ReadDefaults(defs[1])
	assert.DeepEqual(t, defaultArgsMap, map[string]string{
		"FINAL_IMAGE": "alpine:3.17",
		"HOME":        "/root",
	})
}
