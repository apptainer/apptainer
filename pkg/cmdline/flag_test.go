// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cmdline

import (
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/spf13/cobra"
)

var (
	testString      string
	testBool        bool
	testStringSlice []string
	testInt         int
	testUint32      uint32
	testStringMap   map[string]string
)

var ttData = []struct {
	desc       string
	flag       *Flag
	cmd        *cobra.Command
	envValue   string
	matchValue string
	// Alternative match to accommodate random map ordering
	altMatchValue   string
	expectedFailure bool
}{
	{
		desc:            "nil flag",
		cmd:             rootCmd,
		expectedFailure: true,
	},
	{
		desc:            "nil command",
		expectedFailure: true,
	},
	{
		desc: "bad type flag",
		flag: &Flag{
			ID:           "testBadTypeFlag",
			Value:        &testString,
			DefaultValue: &cobra.Command{},
			Name:         "bad-type",
			Usage:        "a bad type flag",
		},
		cmd:             parentCmd,
		expectedFailure: true,
	},
	{
		desc: "string flag",
		flag: &Flag{
			ID:           "testStringFlag",
			Value:        &testString,
			DefaultValue: testString,
			Name:         "string",
			ShortHand:    "s",
			Usage:        "a string flag",
			EnvKeys:      []string{"STRING"},
		},
		cmd:        parentCmd,
		envValue:   "a string",
		matchValue: "a string",
	},
	{
		desc: "string deprecated flag",
		flag: &Flag{
			ID:           "testStringDeprecatedFlag",
			Value:        &testString,
			DefaultValue: testString,
			Deprecated:   "deprecated",
			Name:         "string-dep",
			Usage:        "a deprecated string flag",
		},
		cmd: parentCmd,
	},
	{
		desc: "string hidden flag",
		flag: &Flag{
			ID:           "testStringHiddenFlag",
			Value:        &testString,
			DefaultValue: testString,
			Name:         "string-hidden",
			Usage:        "a hidden string flag",
		},
		cmd: parentCmd,
	},
	{
		desc: "string required flag",
		flag: &Flag{
			ID:           "testStringRequiredFlag",
			Value:        &testString,
			DefaultValue: testString,
			Name:         "string-required",
			Usage:        "a required string flag",
		},
		cmd: parentCmd,
	},
	{
		desc: "boolean flag",
		flag: &Flag{
			ID:           "testBoolFlag",
			Value:        &testBool,
			DefaultValue: testBool,
			Name:         "bool",
			Usage:        "a boolean flag",
			EnvKeys:      []string{"BOOL"},
		},
		cmd:        parentCmd,
		envValue:   "1",
		matchValue: "true",
	},
	{
		desc: "boolean flag (short)",
		flag: &Flag{
			ID:           "testBoolShortFlag",
			Value:        &testBool,
			DefaultValue: testBool,
			Name:         "bool-short",
			ShortHand:    "b",
			Usage:        "a boolean flag (short)",
		},
		cmd: parentCmd,
	},
	{
		desc: "string slice flag",
		flag: &Flag{
			ID:           "testStringSliceFlag",
			Value:        &testStringSlice,
			DefaultValue: testStringSlice,
			Name:         "string-slice",
			Usage:        "a string slice flag",
			EnvKeys:      []string{"STRING_SLICE"},
		},
		cmd:        parentCmd,
		envValue:   "arg1,arg2",
		matchValue: "[arg1,arg2]",
	},
	{
		desc: "string slice flag (short)",
		flag: &Flag{
			ID:           "testStringSliceShortFlag",
			Value:        &testStringSlice,
			DefaultValue: testStringSlice,
			Name:         "string-slice-short",
			ShortHand:    "l",
			Usage:        "a string slice flag (short)",
		},
		cmd: parentCmd,
	},
	{
		desc: "string map flag",
		flag: &Flag{
			ID:           "testStringMapFlag",
			Value:        &testStringMap,
			DefaultValue: testStringMap,
			Name:         "string-map",
			Usage:        "a string map flag",
			EnvKeys:      []string{"STRING_MAP"},
		},
		cmd:           parentCmd,
		envValue:      "key1=arg1,key2=arg2",
		matchValue:    "[key1=arg1,key2=arg2]",
		altMatchValue: "[key2=arg2,key1=arg1]",
	},
	{
		desc: "int flag",
		flag: &Flag{
			ID:           "testIntFlag",
			Value:        &testInt,
			DefaultValue: testInt,
			Name:         "int",
			Usage:        "an int flag",
			EnvKeys:      []string{"INT"},
		},
		cmd:        parentCmd,
		envValue:   "-1234",
		matchValue: "-1234",
	},
	{
		desc: "int flag (short)",
		flag: &Flag{
			ID:           "testIntShortFlag",
			Value:        &testInt,
			DefaultValue: testInt,
			Name:         "int-short",
			ShortHand:    "i",
			Usage:        "an int flag (short)",
		},
		cmd: parentCmd,
	},
	{
		desc: "uint32 flag",
		flag: &Flag{
			ID:           "testUint32Flag",
			Value:        &testUint32,
			DefaultValue: testUint32,
			Name:         "uint",
			Usage:        "a uint32 flag",
			EnvKeys:      []string{"UINT32"},
		},
		cmd:        parentCmd,
		envValue:   "1234",
		matchValue: "1234",
	},
	{
		desc: "uint32 flag (short)",
		flag: &Flag{
			ID:           "testUint32ShortFlag",
			Value:        &testUint32,
			DefaultValue: testUint32,
			Name:         "uint-short",
			ShortHand:    "u",
			Usage:        "a uint32 flag (short)",
		},
		cmd: parentCmd,
	},
}

func TestCmdFlag(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	var c struct{}
	cmds := make(map[*cobra.Command]struct{})

	// create command manager
	cm, err := newCommandManager(rootCmd)
	if err != nil {
		t.Errorf("unexpected error while instantiating new command manager: %err", err)
	}

	// test flag registration
	for _, d := range ttData {
		cm.RegisterFlagForCmd(d.flag, d.cmd)
		if len(cm.GetError()) > 0 && !d.expectedFailure {
			t.Errorf("unexpected failure for %s", d.desc)
		} else if len(cm.GetError()) == 0 && d.expectedFailure {
			t.Errorf("unexpected success for %s", d.desc)
		} else if len(cm.GetError()) == 0 && d.envValue != "" && len(d.flag.EnvKeys) > 0 {
			t.Setenv(d.flag.EnvKeys[0], d.envValue)
			cmds[d.cmd] = c
		}
		// reset error
		cm.errPool = make([]error, 0)
	}

	for cmd := range cmds {
		if err := cm.UpdateCmdFlagFromEnv(cmd, -1, make(map[string]string)); err != nil {
			t.Error(err)
		}
	}

	for _, d := range ttData {
		if d.flag == nil || d.cmd == nil {
			continue
		}
		if d.envValue != "" {
			v := d.cmd.Flags().Lookup(d.flag.Name).Value.String()
			// We allow matching against an optional altMatchValue, so that we can accommodate
			// the string forms of both orderings of maps with 2 keys.
			if v != d.matchValue && (d.altMatchValue == "" || v != d.altMatchValue) {
				t.Errorf("unexpected value for %s, returned %s instead of %s", d.desc, v, d.matchValue)
			}
		}
	}
}
