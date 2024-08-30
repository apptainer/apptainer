// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package files

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

const filenameExpansionScript = `
for n in %[1]s ; do
	echo -n "$n\0"
done
`

func expandPath(path string) ([]string, error) {
	var output, stderr bytes.Buffer

	path = strings.Replace(path, " ", "\\ ", -1)
	cmdline := fmt.Sprintf(filenameExpansionScript, path)
	parser, err := syntax.NewParser().Parse(strings.NewReader(cmdline), "")
	if err != nil {
		return nil, err
	}

	runner, err := interp.New(
		interp.StdIO(nil, &output, &stderr),
	)
	if err != nil {
		return nil, err
	}

	err = runner.Run(context.TODO(), parser)
	if err != nil {
		return nil, err
	}

	// parse expanded output and ignore empty strings from consecutive null bytes
	paths := make([]string, 0, len(strings.Split(output.String(), "\\0")))

	for _, s := range strings.Split(output.String(), "\\0") {
		if s == "" {
			continue
		}
		paths = append(paths, s)
	}

	return paths, nil
}

// joinKeepSlash joins path to prefix, ensuring that if path ends with a "/" it
// is preserved in the result, as may be required when calling out to commands
// for which this is meaningful.
func joinKeepSlash(prefix, path string) string {
	fullPath := filepath.Join(prefix, path)
	// append a slash if path ended with a trailing '/', second check
	// makes sure we don't return a double slash
	if strings.HasSuffix(path, "/") && !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	return fullPath
}

// secureJoinKeepSlash joins path to prefix, but guarantees the resulting path is under prefix.
// If path ends with a "/" it is preserved in the result, as may be required when calling
// out to commands for which this is meaningful.
func secureJoinKeepSlash(prefix, path string) (string, error) {
	fullPath, err := securejoin.SecureJoin(prefix, path)
	if err != nil {
		return "", err
	}
	// append a slash if path ended with a trailing '/', second check
	// makes sure we don't return a double slash
	if strings.HasSuffix(path, "/") && !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	return fullPath, nil
}
