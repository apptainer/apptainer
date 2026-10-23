// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package endpoint

import (
	"net/http"
	"net/http/httptest"
	"testing"

	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
)

func init() {
	useragent.InitValue("apptainer", "0.1.0")
}

func TestConfig_RegistryURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../../../test/remote/config.example.json")
	}))
	t.Cleanup(srv.Close)

	ep := Config{
		URI:      srv.URL,
		Insecure: true,
	}

	expectRegistry := "https://registry.example.com"
	rURI, err := ep.RegistryURI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if rURI != expectRegistry {
		t.Errorf("expected %q, got %q", expectRegistry, rURI)
	}
}
