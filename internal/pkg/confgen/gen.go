// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package confgen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

func Gen(args []string) error {
	switch len(args) {
	case 2:
		inPath := filepath.Clean(args[0])
		outPath := filepath.Clean(args[1])
		return genConf("", inPath, outPath)
	case 1:
		outPath := filepath.Clean(args[0])
		return genConf("", "", outPath)
	default:
		return errors.New("unexpected number of parameters")
	}
}

// genConf produces an apptainer.conf file at out. It retains set configurations from in (leave blank for default)
func genConf(tmpl, in, out string) error {
	inFile := in
	// Parse current apptainer.conf file into c
	if _, err := os.Stat(in); os.IsNotExist(err) {
		inFile = ""
	}
	c, err := apptainerconf.Parse(inFile)
	if err != nil {
		return fmt.Errorf("unable to parse apptainer.conf file: %v", err)
	}

	newOutFile, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("unable to create file %s: %v", out, err)
	}
	defer newOutFile.Close()

	if err := apptainerconf.Generate(newOutFile, tmpl, c); err != nil {
		return fmt.Errorf("unable to generate config file: %v", err)
	}
	return nil
}
