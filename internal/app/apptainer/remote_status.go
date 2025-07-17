// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/apptainer/apptainer/internal/pkg/client/library"
	"github.com/apptainer/apptainer/internal/pkg/remote"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/pkg/sylog"
	containerclient "github.com/apptainer/container-library-client/client"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const statusLine = "%s\t%s\t%s\t%s\n"

type status struct {
	name    string
	uri     string
	status  string
	version string
}

// RemoteStatus checks status of services related to an endpoint
// If the supplied remote name is an empty string, it will attempt
// to use the default remote.
func RemoteStatus(usrConfigFile, name string) (err error) {
	if name != "" {
		sylog.Infof("Checking status of remote: %s", name)
	} else {
		sylog.Infof("Checking status of default remote.")
	}

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
	c, err := remote.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("while parsing remote config data: %s", err)
	}

	if err := syncSysConfig(c); err != nil {
		return err
	}

	var e *endpoint.Config
	if name == "" {
		e, err = c.GetDefault()
	} else {
		e, err = c.GetRemote(name)
	}

	if err != nil {
		return err
	}

	sps, err := e.GetAllServices()
	if err != nil {
		return fmt.Errorf("while retrieving services: %s", err)
	}

	ch := make(chan *status)
	for name, sp := range sps {
		for _, service := range sp {
			go func() {
				ch <- doStatusCheck(name, service)
			}()
		}
	}

	// map storing statuses by name
	smap := make(map[string]*status)
	for _, sp := range sps {
		for range sp {
			s := <-ch
			if s == nil {
				continue
			}
			smap[s.name] = s
		}
	}

	// list in alphanumeric order
	names := make([]string, 0, len(smap))
	for n := range smap {
		names = append(names, n)
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, statusLine, "SERVICE", "STATUS", "VERSION", "URI")
	for _, n := range names {
		s := smap[n]
		fmt.Fprintf(tw, statusLine, cases.Title(language.English).String(s.name), s.status, s.version, s.uri)
	}
	tw.Flush()

	libClientConfigURI := ""
	libClientConfig, err := e.LibraryClientConfig(libClientConfigURI)
	if err != nil {
		return fmt.Errorf("could not get library client configuration: %v", err)
	}

	printLoggedInIdentity(libClientConfig)

	return doTokenCheck(e)
}

func doStatusCheck(name string, sp endpoint.Service) *status {
	uri := sp.URI()
	version, err := sp.Status()
	if err != nil {
		if err == endpoint.ErrStatusNotSupported {
			return nil
		}
		return &status{name: name, uri: uri, status: "N/A"}
	}
	return &status{name: name, uri: uri, status: "OK", version: version}
}

func printLoggedInIdentity(config *containerclient.Config) {
	username, email, err := library.GetIdentity(config)

	if err == nil && len(username) > 0 {
		fmt.Printf("\nLogged in as: %s <%s>\n\n", username, email)
	}
}

func doTokenCheck(e *endpoint.Config) error {
	if e.Token == "" {
		fmt.Println("\nNo authentication token set (logged out).")
		return nil
	}
	if err := e.VerifyToken(""); err != nil {
		fmt.Println("\nAuthentication token is invalid (please login again).")
		return err

	}
	fmt.Println("\nValid authentication token set (logged in).")
	return nil
}
