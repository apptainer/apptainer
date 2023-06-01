package cli

import (
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/build"
	"gotest.tools/v3/assert"
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
			err:    "is not `key=value` pair format",
		},
		{
			name:   "wrong case because of missing key",
			input:  "\n  =v1\n",
			expect: []string{},
			err:    "key field is missing in text:",
		},
		{
			name:   "wrong case because of missing value",
			input:  "\n  k1 =\n",
			expect: []string{},
			err:    "value field is missing in text:",
		},
		{
			name:   "wrong case because of multiple ==",
			input:  "\n  k1 == v1\n",
			expect: []string{},
			err:    "is not `key=value` pair format",
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

func TestReplaceVar(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		output      string
		argsMap     map[string]string
		deffArgsMap map[string]string
		err         string
	}{
		{
			name:   "normal case",
			input:  "/script-{{ APP_VER }}",
			output: "/script-1.0",
			argsMap: map[string]string{
				"OS_VER":  "1",
				"APP_VER": "1.0",
			},
			deffArgsMap: map[string]string{},
			err:         "",
		},
		{
			name:   "normal case 2",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "/script-1.sh 1.0",
			argsMap: map[string]string{
				"OS_VER":  "1",
				"APP_VER": "1.0",
			},
			deffArgsMap: map[string]string{},
			err:         "",
		},
		{
			name:        "normal case 3",
			input:       "/script-1.sh 1.0",
			output:      "",
			argsMap:     map[string]string{},
			deffArgsMap: map[string]string{},
			err:         "no change to text",
		},
		{
			name:   "normal case 4",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "/script-1.sh 1.0",
			argsMap: map[string]string{
				"OS_VER": "1",
			},
			deffArgsMap: map[string]string{
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
			deffArgsMap: map[string]string{
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
			deffArgsMap: map[string]string{},
			err:         "",
		},
		{
			name:   "wrong case because of missing variable",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "",
			argsMap: map[string]string{
				"OS_VER": "1",
			},
			deffArgsMap: map[string]string{},
			err:         "is not defined through either --build-arg (--build-arg-file) or 'arguments' section",
		},
		{
			name:   "wrong case because of missing variable 2",
			input:  "/script-{{ OS_VER }}.sh {{ APP_VER }}",
			output: "",
			argsMap: map[string]string{
				"OS_VE": "1",
			},
			deffArgsMap: map[string]string{},
			err:         "is not defined through either --build-arg (--build-arg-file) or 'arguments' section",
		},
	}

	for _, test := range tests {
		t.Logf("Starting %s", test.name)
		output, _, err := replaceVar([]byte(test.input), test.argsMap, test.deffArgsMap)
		if test.err != "" {
			assert.ErrorContains(t, err, test.err)
		} else {
			assert.Equal(t, string(output), test.output)
		}
	}
}

func TestProcessDefsSingleDef(t *testing.T) {
	d, err := build.MakeAllDefs("../../../e2e/testdata/build-template/single-stage-unit-test.def")
	assert.NilError(t, err)

	args := []string{
		"OS_VER=1",
		"AUTHOR=jason",
	}

	d, err = processDefs(args, "", d)
	assert.NilError(t, err)
	assert.Equal(t, d[0].Header["from"], "alpine:1")
	assert.Equal(t, strings.TrimSpace(d[0].Help.Script), "This is a demo for templating definition file")
	assert.Equal(t, d[0].Labels["Author"], "jason")
	assert.Equal(t, strings.TrimSpace(d[0].Environment.Script), "export OS_VER=1")
}

func TestProcessDefsMultipleDef(t *testing.T) {
	d, err := build.MakeAllDefs("../../../e2e/testdata/build-template/multiple-stage-unit-test.def")
	assert.NilError(t, err)

	args := []string{
		"DEVEL_IMAGE=alpine:3.9",
		"FINAL_IMAGE=alpine:3.17",
	}

	d, err = processDefs(args, "", d)
	assert.NilError(t, err)
	assert.Equal(t, d[0].Header["from"], "alpine:3.9")
	rt := strings.Contains(d[0].BuildData.Post.Script, "export HOME=/root")
	assert.Equal(t, rt, true)
	rt = strings.Contains(d[0].BuildData.Post.Script, "cd /root")
	assert.Equal(t, rt, true)

	assert.Equal(t, d[1].Header["from"], "alpine:3.17")
	rt = strings.Contains(d[1].BuildData.Files[0].Files[0].Src, "/root/hello")
	assert.Equal(t, rt, true)
}

func TestProcessWithAdditionalArgs(t *testing.T) {
	d, err := build.MakeAllDefs("../../../e2e/testdata/build-template/single-stage-unit-test.def")
	assert.NilError(t, err)

	args := []string{
		"OS_VER=1",
		"AUTHOR=jason",
		"ADDITION=1",
	}

	_, err = processDefs(args, "", d)
	assert.ErrorContains(t, err, "there are unmatched ADDITION variables")
}
