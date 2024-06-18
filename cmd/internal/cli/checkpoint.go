// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/checkpoint/dmtcp"
	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

const listLine = "%s\n"

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(CheckpointCmd)
		cmdManager.RegisterSubCmd(CheckpointCmd, CheckpointListCmd)
		cmdManager.RegisterSubCmd(CheckpointCmd, CheckpointInstanceCmd)
		cmdManager.RegisterSubCmd(CheckpointCmd, CheckpointCreateCmd)
		cmdManager.RegisterSubCmd(CheckpointCmd, CheckpointDeleteCmd)

		cmdManager.RegisterFlagForCmd(&actionHomeFlag, CheckpointInstanceCmd)
	})
}

func checkpointPreRun(_ *cobra.Command, _ []string) {
	dmtcp.QuickInstallationCheck()
}

// CheckpointCmd represents the checkpoint command.
var CheckpointCmd = &cobra.Command{
	Run: nil,

	Use:                   docs.CheckpointUse,
	Short:                 docs.CheckpointShort,
	Long:                  docs.CheckpointLong,
	Example:               docs.CheckpointExample,
	DisableFlagsInUseLine: true,
}

// CheckpointListCmd apptainer checkpoint list
var CheckpointListCmd = &cobra.Command{
	Args:   cobra.ExactArgs(0),
	PreRun: checkpointPreRun,
	Run: func(_ *cobra.Command, _ []string) {
		m := dmtcp.NewManager()

		entries, err := m.List()
		if err != nil {
			sylog.Fatalf("Failed to get checkpoint entries: %v", err)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, listLine, "NAME")

		for _, e := range entries {
			fmt.Fprintf(tw, listLine, filepath.Base(e.Path()))
		}

		tw.Flush()
	},

	Use:     docs.CheckpointListUse,
	Short:   docs.CheckpointListShort,
	Long:    docs.CheckpointListLong,
	Example: docs.CheckpointListExample,

	DisableFlagsInUseLine: true,
}

// CheckpointCreateCmd apptainer checkpoint create
var CheckpointCreateCmd = &cobra.Command{
	Args:   cobra.ExactArgs(1),
	PreRun: checkpointPreRun,
	Run: func(_ *cobra.Command, args []string) {
		name := args[0]
		m := dmtcp.NewManager()

		_, err := m.Get(name)
		if err == nil {
			sylog.Fatalf("Checkpoint %q already exists.", name)
		}

		_, err = m.Create(name)
		if err != nil {
			sylog.Fatalf("Failed to create checkpoint: %s", err)
		}

		sylog.Infof("Checkpoint %q created.", name)
	},

	Use:     docs.CheckpointCreateUse,
	Short:   docs.CheckpointCreateShort,
	Long:    docs.CheckpointCreateLong,
	Example: docs.CheckpointCreateExample,

	DisableFlagsInUseLine: true,
}

// CheckpointDeleteCmd apptainer checkpoint delete
var CheckpointDeleteCmd = &cobra.Command{
	Args:   cobra.ExactArgs(1),
	PreRun: checkpointPreRun,
	Run: func(_ *cobra.Command, args []string) {
		name := args[0]
		m := dmtcp.NewManager()

		err := m.Delete(name)
		if err != nil {
			sylog.Fatalf("Failed to delete checkpoint entries: %v", err)
		}

		sylog.Infof("Checkpoint %q deleted.", name)
	},

	Use:     docs.CheckpointDeleteUse,
	Short:   docs.CheckpointDeleteShort,
	Long:    docs.CheckpointDeleteLong,
	Example: docs.CheckpointDeleteExample,

	DisableFlagsInUseLine: true,
}

var CheckpointInstanceCmd = &cobra.Command{
	Args: cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		checkpointPreRun(cmd, args)
		actionPreRun(cmd, args)
	},
	Run: func(cmd *cobra.Command, args []string) {
		instanceName := args[0]

		file, err := instance.Get(instanceName, instance.AppSubDir)
		if err != nil {
			sylog.Fatalf("Could not retrieve instance file: %s", err)
		}

		if file.Checkpoint == "" {
			sylog.Fatalf("This instance was not started with checkpointing.")
		}

		m := dmtcp.NewManager()

		e, err := m.Get(file.Checkpoint)
		if err != nil {
			sylog.Fatalf("Failed to get checkpoint entry: %v", err)
		}

		port, err := e.CoordinatorPort()
		if err != nil {
			sylog.Fatalf("Failed to parse port file for coordinator port: %s", err)
		}

		sylog.Infof("Using checkpoint %q", e.Name())

		a := append([]string{"/.singularity.d/actions/exec"}, dmtcp.CheckpointArgs(port)...)
		if err := launchContainer(cmd, "instance://"+args[0], a, "", -1); err != nil {
			sylog.Fatalf("%s", err)
		}
	},

	Use:     docs.CheckpointInstanceUse,
	Short:   docs.CheckpointInstanceShort,
	Long:    docs.CheckpointInstanceLong,
	Example: docs.CheckpointInstanceExample,

	DisableFlagsInUseLine: true,
}
