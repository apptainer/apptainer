// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/remote"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/internal/pkg/test"
	"gopkg.in/yaml.v3"
)

const (
	invalidCfgFilePath = "/not/a/real/file"
	invalidURI         = "really//not/a/URI"
	validURI           = "cloud.random.io"
	validRemoteName    = "cloud_testing"
)

func createInvalidCfgFile(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "invalid.yml")

	// Set an invalid configuration
	type aDummyStruct struct {
		NoneSenseRemote string
	}
	cfg := aDummyStruct{
		NoneSenseRemote: "toto",
	}

	yaml, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("cannot marshal YAML: %s\n", err)
	}

	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func createValidCfgFile(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "valid.yml")

	// Set a valid configuration
	cfg := remote.Config{
		DefaultRemote: validRemoteName,
		Remotes: map[string]*endpoint.Config{
			"random": {
				URI:   "validURI",
				Token: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.TCYt5XsITJX1CxPCT8yAV-TVkIEq_PbChOMqsLfRoPsnsgw5WEuts01mq-pQy7UJiN5mgRxD-WUcX16dUEMGlv50aqzpqh4Qktb3rk-BuQy72IFLOqV0G_zS245-kronKb78cPN25DGlcTwLtjPAYuNzVBAh4vGHSrQyHUdBBPM",
			},
			"cloud": {
				URI:   "validURI",
				Token: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.TCYt5XsITJX1CxPCT8yAV-TVkIEq_PbChOMqsLfRoPsnsgw5WEuts01mq-pQy7UJiN5mgRxD-WUcX16dUEMGlv50aqzpqh4Qktb3rk-BuQy72IFLOqV0G_zS245-kronKb78cPN25DGlcTwLtjPAYuNzVBAh4vGHSrQyHUdBBPM",
			},
		},
	}

	yaml, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("cannot marshal YAML: %s\n", err)
	}

	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

//nolint:maintidx
func TestRemoteAdd(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	validCfgFile := createValidCfgFile(t)
	defer os.Remove(validCfgFile)

	invalidCfgFile := createInvalidCfgFile(t)
	defer os.Remove(invalidCfgFile)

	tests := []struct {
		name        string
		cfgfile     string
		remoteName  string
		uri         string
		global      bool
		insecure    bool
		makeDefault bool
		shallPass   bool
	}{
		{
			name:        "1: invalid config file; empty remote name; invalid URI, local; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "2: invalid config file; empty remote name; empty URI; local; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         "",
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "3: invalid config file; valid remote name; invalid URI; local; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "4: invalid config file; valid remote name; empty URI; local; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "5: valid config file; empty remote name; invalid URI, local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "6: valid config file; empty remote name; empty URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         "",
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "7: valid config file; valid remote name; empty URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "8: valid config file; valid remote name; invalid URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: false,
			shallPass:   true,
		},
		{
			// This test checks both RemoteAdd() and RemoteRemove(), we still
			// have a separate test for corner cases in the context of
			// RemoveRemove().
			name:        "9: valid config file; valid remote name; valid URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "10: valid config file; valid remote name; empty URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "11: valid config file; empty remote name; valid URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         validURI,
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "12: valid config file; valid remote name; invalid URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "13: valid config file: valid remote name; valid URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "14: invalid config file; empty remote name; invalid URI, global; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "15: invalid config file; empty remote name; empty URI; global; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         "",
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "16: invalid config file; valid remote name; invalid URI; global; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "17: invalid config file; valid remote name; invalid URI; global; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "18: valid config file; empty remote name; invalid URI, global; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "19: valid config file; empty remote name; empty URI; global; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         "",
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "20: valid config file; valid remote name; invalid URI; global; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      true,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "21: valid config file; valid remote name; invalid URI; global; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "22: invalid config file path; valid remote name; invalid URI; local; notDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "23: invalid config file path; empty remote name; invalid URI; global; notDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  "",
			uri:         invalidURI,
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "24: invalid config file path; valid remote name; invalid URI; local; notDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "25: invalid config file path; empty remote name; invalid URI; global; notDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  "",
			uri:         "",
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "26: valid config file; valid remote name; valid URI; global; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      true,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "27: valid config file; empty remote name; valid URI; global; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         validURI,
			global:      true,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "28: invalid config file; empty remote name; valid URI; local; notDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         validURI,
			global:      false,
			makeDefault: false,
			shallPass:   false,
		},
		{
			name:        "29: invalid config file: valid remote name; valid URI; local; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "30: valid config file; valid remote name; valid URI; local; insecure; notDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			insecure:    true,
			makeDefault: false,
			shallPass:   true,
		},
		{
			name:        "31: invalid config file; empty remote name; invalid URI, local; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "32: invalid config file; empty remote name; empty URI; local; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         "",
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "33: invalid config file; valid remote name; invalid URI; local; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "34: invalid config file; valid remote name; empty URI; local; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "35: valid config file; empty remote name; invalid URI, local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "36: valid config file; empty remote name; empty URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         "",
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "37: valid config file; valid remote name; empty URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "38: valid config file; valid remote name; invalid URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: true,
			shallPass:   true,
		},
		{
			// This test checks both RemoteAdd() and RemoteRemove(), we still
			// have a separate test for corner cases in the context of
			// RemoveRemove().
			name:        "39: valid config file; valid remote name; valid URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			makeDefault: true,
			shallPass:   true,
		},
		{
			name:        "40: valid config file; valid remote name; empty URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "41: valid config file; empty remote name; valid URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         validURI,
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "42: valid config file; valid remote name; invalid URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: true,
			shallPass:   true,
		},
		{
			name:        "43: valid config file: valid remote name; valid URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			makeDefault: true,
			shallPass:   true,
		},
		{
			name:        "44: invalid config file; empty remote name; invalid URI, global; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "45: invalid config file; empty remote name; empty URI; global; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         "",
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "46: invalid config file; valid remote name; invalid URI; global; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "47: invalid config file; valid remote name; invalid URI; global; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "48: valid config file; empty remote name; invalid URI, global; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         invalidURI,
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "49: valid config file; empty remote name; empty URI; global; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         "",
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "50: valid config file; valid remote name; invalid URI; global; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      true,
			makeDefault: true,
			shallPass:   true,
		},
		{
			name:        "51: valid config file; valid remote name; invalid URI; global; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         "",
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "52: invalid config file path; valid remote name; invalid URI; local; makeDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  validRemoteName,
			uri:         invalidURI,
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "53: invalid config file path; empty remote name; invalid URI; global; makeDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  "",
			uri:         invalidURI,
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "54: invalid config file path; valid remote name; invalid URI; local; makeDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  validRemoteName,
			uri:         "",
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "55: invalid config file path; empty remote name; invalid URI; global; makeDefault",
			cfgfile:     invalidCfgFilePath,
			remoteName:  "",
			uri:         "",
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "56: valid config file; valid remote name; valid URI; global; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      true,
			makeDefault: true,
			shallPass:   true,
		},
		{
			name:        "57: valid config file; empty remote name; valid URI; global; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  "",
			uri:         validURI,
			global:      true,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "58: invalid config file; empty remote name; valid URI; local; makeDefault",
			cfgfile:     invalidCfgFile,
			remoteName:  "",
			uri:         validURI,
			global:      false,
			makeDefault: true,
			shallPass:   false,
		},
		{
			name:        "59: invalid config file: valid remote name; valid URI; local; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			makeDefault: true,
			shallPass:   true,
		},
		{
			name:        "60: valid config file; valid remote name; valid URI; local; insecure; makeDefault",
			cfgfile:     validCfgFile,
			remoteName:  validRemoteName,
			uri:         validURI,
			global:      false,
			insecure:    true,
			makeDefault: true,
			shallPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.DropPrivilege(t)
			defer test.ResetPrivilege(t)

			// remote package detect if the configuration file is
			// the system configuration to apply some restriction,
			// therefore we need to replace SystemConfigPath in
			// the remote package in order to make the test configuration
			// file as the system configuration file. This shouldn't interfere
			// with remove tests in remote_remove_test.go (hopefully)
			oldSysConfig := ""
			restoreSysConfig := func() {
				if oldSysConfig != "" {
					remote.SystemConfigPath = oldSysConfig
				}
			}

			if tt.global && tt.shallPass {
				oldSysConfig = remote.SystemConfigPath
				remote.SystemConfigPath = tt.cfgfile
			}

			err := RemoteAdd(tt.cfgfile, tt.remoteName, tt.uri, tt.global, tt.insecure, tt.makeDefault)
			if tt.shallPass == true && err != nil {
				restoreSysConfig()
				t.Fatalf("valid case failed: %s\n", err)
			}

			if tt.shallPass == false && err == nil {
				restoreSysConfig()
				RemoteRemove(tt.cfgfile, tt.remoteName)
				t.Fatal("invalid case passed")
			}

			if tt.shallPass == true && err == nil {
				RemoteRemove(tt.cfgfile, tt.remoteName)
				restoreSysConfig()
			}
		})
	}
}
