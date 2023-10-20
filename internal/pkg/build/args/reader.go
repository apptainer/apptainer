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
	"fmt"
	"io"
	"regexp"

	"github.com/samber/lo"
)

var (
	buildArgsRegexp   = regexp.MustCompile(`{{\s*(\w+)\s*}}`)
	commentLineRegexp = regexp.MustCompile(`\s*[#][^!]\s*.*`)
)

// NewReader creates a io.Reader that will provide the contents of a def file
// with build-args replacements applied. src is an io.Reader from which the
// pre-replacement def file will be read. buildArgsMap provides the replacements
// requested by the user, and defaultArgsMap provides the replacements specified
// in the %arguments section of the def file (or build stage). The arguments
// actually encountered in the course of the replacement will be appended to the
// slice designated by consumedArgs.
func NewReader(src io.Reader, buildArgsMap map[string]string, defaultArgsMap map[string]string, consumedArgs *[]string) (io.Reader, error) {
	srcBytes, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// do templating
	matches := buildArgsRegexp.FindAllSubmatchIndex(srcBytes, -1)
	mapOfConsumedArgs := make(map[string]bool)
	var buf bytes.Buffer
	bufWriter := io.Writer(&buf)
	i := 0
	for _, m := range matches {
		// find the last newline
		newlineIdx := i
		for start := m[0]; start >= newlineIdx; start-- {
			if srcBytes[start] == '\n' {
				newlineIdx = start
				break
			}
		}

		// check whether current line containing {{ VAR }} is commented line
		if commentLineRegexp.Match(srcBytes[newlineIdx:m[0]]) {
			continue
		}

		bufWriter.Write(srcBytes[i:m[0]])
		argName := string(srcBytes[m[2]:m[3]])
		val, ok := buildArgsMap[argName]
		if !ok {
			val, ok = defaultArgsMap[argName]
		}
		if !ok {
			return nil, fmt.Errorf("build var %s is not defined through either --build-arg (--build-arg-file) or 'arguments' section", argName)
		}
		bufWriter.Write([]byte(val))
		mapOfConsumedArgs[argName] = true
		i = m[1]
	}
	bufWriter.Write(srcBytes[i:])

	*consumedArgs = append(*consumedArgs, lo.Keys(mapOfConsumedArgs)...)

	r := bytes.NewReader(buf.Bytes())

	return r, nil
}
