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
	"io"
	"os"

	"github.com/apptainer/apptainer/internal/pkg/remote"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// RemoteLogout logs out from an endpoint.
func RemoteLogout(usrConfigFile, name string) (err error) {
	// opening config file
	file, err := os.OpenFile(usrConfigFile, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("while opening remote config file: %s", err)
	}
	defer file.Close()

	// read file contents to config struct
	c, err := remote.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("while parsing remote config data: %s", err)
	}

	if err := syncSysConfig(c); err != nil {
		return err
	}

	var r *endpoint.Config
	if name == "" {
		r, err = c.GetDefault()
	} else {
		r, err = c.GetRemote(name)
	}

	if r != nil {
		// endpoint
		r.Token = ""
	} else {
		// services
		sylog.Warningf("'remote logout' is deprecated for registries or keyservers and will be removed in a future release; running 'registry logout'")
		return RegistryLogout(usrConfigFile, name, "")
	}

	// truncating file before writing new contents and syncing to commit file
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("while truncating remote config file: %s", err)
	}

	if n, err := file.Seek(0, io.SeekStart); err != nil || n != 0 {
		return fmt.Errorf("failed to reset %s cursor: %s", file.Name(), err)
	}

	if _, err := c.WriteTo(file); err != nil {
		return fmt.Errorf("while writing remote config to file: %s", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to flush remote config file %s: %s", file.Name(), err)
	}

	return nil
}

func CommonLoggout(usrConfigFile, name string, reqAuthFile string) (err error) {
	// opening config file
	file, err := os.OpenFile(usrConfigFile, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("while opening configuration file: %s", err)
	}
	defer file.Close()

	// read file contents to config struct
	c, err := remote.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("while parsing configuration data: %s", err)
	}

	if err := syncSysConfig(c); err != nil {
		return err
	}

	// services
	if err := c.Logout(name, reqAuthFile); err != nil {
		return fmt.Errorf("while verifying token: %v", err)
	}

	// truncating file before writing new contents and syncing to commit file
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("while truncating configuration file: %s", err)
	}

	if n, err := file.Seek(0, io.SeekStart); err != nil || n != 0 {
		return fmt.Errorf("failed to reset %s cursor: %s", file.Name(), err)
	}

	if _, err := c.WriteTo(file); err != nil {
		return fmt.Errorf("while writing configuration to file: %s", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to flush configuration file %s: %s", file.Name(), err)
	}

	return nil
}
