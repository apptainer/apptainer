package build

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/pkg/build/types/parser"
	"golang.org/x/text/transform"
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
			err:    "is not a key=value pair",
		},
		{
			name:   "wrong case because of missing key",
			input:  "\n  =v1\n",
			expect: []string{},
			err:    "missing key portion in",
		},
		{
			name:   "wrong case because of missing value",
			input:  "\n  k1 =\n",
			expect: []string{},
			err:    "missing value portion in",
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

func TestTransformer(t *testing.T) {
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
	}

	for _, test := range tests {
		t.Logf("Starting %s", test.name)
		var consumedArgs []string
		transformer := buildArgsTransformer{
			buildArgsMap:   test.argsMap,
			defaultArgsMap: test.defaultArgsMap,
			consumedArgs:   &consumedArgs,
		}
		reader := transform.NewReader(bytes.NewReader([]byte(test.input)), transformer)
		output, err := io.ReadAll(reader)
		if test.err != "" {
			assert.ErrorContains(t, err, test.err)
		} else {
			assert.Equal(t, string(output), test.output)
		}
	}
}

func TestProcessDefsSingleDef(t *testing.T) {
	d, err := MakeAllDefs(
		filepath.Join("..", "..", "..", "test", "build-args", "single-stage-unit-test.def"),
		map[string]string{
			"OS_VER": "1",
			"AUTHOR": "jason",
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, d[0].Header["from"], "alpine:1")
	assert.Equal(t, strings.TrimSpace(d[0].Help.Script), "This is a demo for templating definition file")
	assert.Equal(t, d[0].Labels["Author"], "jason")
	assert.Equal(t, strings.TrimSpace(d[0].Environment.Script), "export OS_VER=1")
}

func TestProcessDefsMultipleDef(t *testing.T) {
	d, err := MakeAllDefs(
		filepath.Join("..", "..", "..", "test", "build-args", "multiple-stage-unit-test.def"),
		map[string]string{
			"DEVEL_IMAGE": "golang:1.12.3-alpine3.9",
			"FINAL_IMAGE": "alpine:3.9",
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, d[0].Header["from"], "golang:1.12.3-alpine3.9")
	rt := strings.Contains(d[0].BuildData.Post.Script, "export HOME=/root")
	assert.Equal(t, rt, true)
	rt = strings.Contains(d[0].BuildData.Post.Script, "cd /root")
	assert.Equal(t, rt, true)

	assert.Equal(t, d[1].Header["from"], "alpine:3.9")
	rt = strings.Contains(d[1].BuildData.Files[0].Files[0].Src, "/root/hello")
	assert.Equal(t, rt, true)
}

func TestReadDefaultArgs(t *testing.T) {
	defFilePath := filepath.Join("..", "..", "..", "test", "build-args", "single-stage-unit-test.def")
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
	defaultArgsMap := readDefaultArgs(defs[0])
	assert.DeepEqual(t, defaultArgsMap, map[string]string{
		"OS_VER": "3.17",
		"DEMO":   "a demo",
		"AUTHOR": "jason",
	})

	defFilePath = filepath.Join("..", "..", "..", "test", "build-args", "multiple-stage-unit-test.def")
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
	defaultArgsMap = readDefaultArgs(defs[0])
	assert.DeepEqual(t, defaultArgsMap, map[string]string{
		"HOME": "/root",
	})
	defaultArgsMap = readDefaultArgs(defs[1])
	assert.DeepEqual(t, defaultArgsMap, map[string]string{
		"FINAL_IMAGE": "alpine:3.17",
		"HOME":        "/root",
	})
}
