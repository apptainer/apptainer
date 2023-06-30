package build

import (
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestProcessDefsSingleDef(t *testing.T) {
	d, _, err := MakeAllDefs(
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
	d, _, err := MakeAllDefs(
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

func TestProcessWithAdditionalArgs(t *testing.T) {
	_, unusedArgs, err := MakeAllDefs(
		filepath.Join("..", "..", "..", "test", "build-args", "single-stage-unit-test.def"),
		map[string]string{
			"OS_VER":   "1",
			"AUTHOR":   "jason",
			"ADDITION": "1",
		},
	)
	assert.NilError(t, err)
	assert.Equal(t, len(unusedArgs), 1)
	assert.Equal(t, "ADDITION", unusedArgs[0])
}
