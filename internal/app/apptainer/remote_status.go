// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/apptainer/apptainer/internal/pkg/remote"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/pkg/sylog"
	scslibclient "github.com/apptainer/container-library-client/client"
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
		name := name
		for _, service := range sp {
			service := service
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
	if err := printLoggedInIdentity(libClientConfig); err != nil {
		return err
	}

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

func printLoggedInIdentity(config *scslibclient.Config) error {
	path := userServicePath
	endPoint := config.BaseURL + path

	req, err := http.NewRequest(http.MethodGet, endPoint, nil)
	if err != nil {
		return fmt.Errorf("error creating new request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", config.AuthToken))
	res, err := config.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		sylog.Debugf("Status Code: %v", res.StatusCode)
		if res.StatusCode == http.StatusUnauthorized {
			// This simply means we are not logged in, which in this context is not an error condition.
			return nil
		} else if res.StatusCode == http.StatusNotFound {
			sylog.Warningf("endpoint for retrieving logged-in user's username/email is not available on the current remote library")
			return nil
		} else {
			return fmt.Errorf("encountered error while trying to retrieve logged-in user's username/email: %v", res.StatusCode)
		}
	}

	var ud userData
	if err := json.NewDecoder(res.Body).Decode(&ud); err != nil {
		return fmt.Errorf("error decoding json response: %v", err)
	}

	if len(ud.Username) > 0 {
		// TODO: Right now, it doesn't seem like the realname field ever contains anything different than the username field, so it would be redundant to print them both. If this ever changes, we can add another %s to the format string and output ud.Realname, as well. (It's already in the struct.)
		fmt.Printf("\nLogged in as: %s <%s>\n\n", ud.Username, ud.Email)
	}

	return nil
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
