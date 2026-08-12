// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// helpColumns is the width that docs.HelpTemplate wraps flag usage output to.
const helpColumns = 80

// TestFlagUsageWrapping checks that the flag usage shown by --help fits within
// helpColumns.
//
// pflag stops wrapping the remainder of a usage string as soon as it meets a
// word that is wider than the description column, so a long example in the
// middle of a usage string silently turns wrapping off for all the text that
// follows it.
func TestFlagUsageWrapping(t *testing.T) {
	// Init is not idempotent, and another test in this package may have
	// called it already.
	if ExecCmd.Flags().Lookup("mount") == nil {
		Init(false)
	}

	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"exec", ExecCmd},
		{"shell", ShellCmd},
		{"run", RunCmd},
		{"build", buildCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := tt.cmd.LocalFlags().FlagUsagesWrapped(helpColumns)
			for _, line := range strings.Split(usage, "\n") {
				line = strings.TrimRight(line, " ")
				if len(line) > helpColumns {
					t.Errorf("flag usage is %d columns, expected at most %d:\n%s",
						len(line), helpColumns, line)
				}
			}
		})
	}
}
