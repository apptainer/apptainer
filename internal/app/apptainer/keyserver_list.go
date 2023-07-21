// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/apptainer/apptainer/internal/pkg/remote"
)

// KeyserverList prints information about remote configurations
func KeyserverList(remoteName string, usrConfigFile string) (err error) {
	c := &remote.Config{}

	// opening config file
	file, err := os.OpenFile(usrConfigFile, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no remote configurations")
		}
		return fmt.Errorf("while opening remote config file: %s", err)
	}
	defer file.Close()

	// read file contents to config struct
	c, err = remote.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("while parsing remote config data: %s", err)
	}

	if err := syncSysConfig(c); err != nil {
		return err
	}

	for epName, ep := range c.Remotes {
		fmt.Println()
		isSystem := ""
		if ep.System {
			isSystem = "*"
		}
		fmt.Printf("%s%s\n", epName, isSystem)

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if err := ep.UpdateKeyserversConfig(); err != nil {
			fmt.Fprintln(tw, " \t(unable to fetch associated keyserver info for this endpoint)")
		}

		order := 1
		for _, kc := range ep.Keyservers {
			if kc.Skip {
				continue
			}
			secure := "🔒"
			if kc.Insecure {
				secure = ""
			}
			fmt.Fprintf(tw, " \t#%d\t%s\t%s\n", order, kc.URI, secure)
			order++
		}
		tw.Flush()
	}

	fmt.Println()
	fmt.Println("(* = system endpoint)")

	return nil
}
