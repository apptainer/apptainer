// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"fmt"

	"github.com/apptainer/apptainer/cmd/internal/cli"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"golang.org/x/sys/unix"
)

func assertAccess(dir string) {
	if err := unix.Access(dir, unix.W_OK); err != nil {
		sylog.Fatalf("Given directory (%s) does not exist or is not writable by calling user", dir)
	}
}

func markdownDocs(rootCmd *cobra.Command, outDir string) {
	assertAccess(outDir)
	sylog.Infof("Creating Apptainer markdown docs at %s\n", outDir)
	if err := doc.GenMarkdownTree(rootCmd, outDir); err != nil {
		sylog.Fatalf("Failed to create markdown docs for apptainer\n")
	}
}

func manDocs(rootCmd *cobra.Command, outDir string) {
	assertAccess(outDir)
	sylog.Infof("Creating Apptainer man pages at %s\n", outDir)
	header := &doc.GenManHeader{
		Title:   "apptainer",
		Section: "1",
	}

	// works recursively on all sub-commands (thanks bauerm97)
	if err := doc.GenManTree(rootCmd, header, outDir); err != nil {
		sylog.Fatalf("Failed to create man pages for apptainer\n")
	}
}

func rstDocs(rootCmd *cobra.Command, outDir string) {
	assertAccess(outDir)
	sylog.Infof("Creating Apptainer RST docs at %s\n", outDir)
	if err := doc.GenReSTTreeCustom(rootCmd, outDir, func(_ string) string {
		return ""
	}, func(name, ref string) string {
		return fmt.Sprintf(":ref:`%s <%s>`", name, ref)
	}); err != nil {
		sylog.Fatalf("Failed to create RST docs for apptainer\n")
	}
}

func main() {
	var dir string
	rootCmd := &cobra.Command{
		ValidArgs: []string{"markdown", "man", "rst"},
		Args:      cobra.ExactArgs(1),
		Use:       "makeDocs {markdown | man | rst}",
		Short:     "Generates Apptainer documentation",
		Run: func(_ *cobra.Command, args []string) {
			// We must Init() as loading commands etc. is deferred until this is called.
			// Using true here will result in local docs including any content for installed
			// plugins.
			cli.Init(true)
			rootCmd := cli.RootCmd()
			switch args[0] {
			case "markdown":
				markdownDocs(rootCmd, dir)
			case "man":
				manDocs(rootCmd, dir)
			case "rst":
				rstDocs(rootCmd, dir)
			default:
				sylog.Fatalf("Invalid output type %s\n", args[0])
			}
		},
	}
	rootCmd.Flags().StringVarP(&dir, "dir", "d", ".", "Directory in which to put the generated documentation")
	rootCmd.Execute()
}
