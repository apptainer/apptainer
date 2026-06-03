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
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

func expandPath(path string) ([]string, error) {
	path = strings.ReplaceAll(path, " ", "\\ ")
	parsedPath, err := syntax.NewParser().Document(strings.NewReader(path))
	if err != nil {
		return nil, err
	}

	expandConfig := expand.Config{
		ExtGlob:  true,
		GlobStar: true,
		ReadDir2: os.ReadDir,
	}

	expandedPaths := expand.FieldsSeq(&expandConfig, parsedPath)

	paths := make([]string, 0, 5)
	for p, err := range expandedPaths {
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
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
