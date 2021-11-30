// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
package main

import (
	"fmt"
	"go/build"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("E: missing target Go version\n")
		os.Exit(128)
	}

	for _, tag := range build.Default.ReleaseTags {
		if tag == os.Args[1] {
			fmt.Printf("Found Go release tag %s.\n", tag)
			return
		}
	}

	fmt.Printf("Go release tag %s not found.\n", os.Args[1])

	os.Exit(1)
}
