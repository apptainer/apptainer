// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

func main() {
	switch len(os.Args) {
	case 3:
		inPath := filepath.Clean(os.Args[1])
		outPath := filepath.Clean(os.Args[2])
		genConf("", inPath, outPath)
	case 2:
		outPath := filepath.Clean(os.Args[1])
		genConf("", "", outPath)
	default:
		fmt.Println("Usage: go run ... [infile] <outfile>")
		os.Exit(1)
	}
}

// genConf produces an apptainer.conf file at out. It retains set configurations from in (leave blank for default)
func genConf(tmpl, in, out string) {
	inFile := in
	// Parse current apptainer.conf file into c
	if _, err := os.Stat(in); os.IsNotExist(err) {
		inFile = ""
	}
	c, err := apptainerconf.Parse(inFile)
	if err != nil {
		fmt.Printf("Unable to parse apptainer.conf file: %s\n", err)
		os.Exit(1)
	}

	newOutFile, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Printf("Unable to create file %s: %v\n", out, err)
	}
	defer newOutFile.Close()

	if err := apptainerconf.Generate(newOutFile, tmpl, c); err != nil {
		fmt.Printf("Unable to generate config file: %v\n", err)
		os.Exit(1)
	}
}
