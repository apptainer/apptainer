// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainerconf

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSplitDirectiveValue(t *testing.T) {
	tests := []struct {
		name      string
		directive string
		value     string
		want      []string
	}{
		{"Default", "bind path", "/etc/localtime,/etc/hosts", []string{"/etc/localtime", "/etc/hosts"}},
		{"Options", "bind path", "/a::dev,suid", []string{"/a::dev,suid"}},
		{"OptionsThenPath", "bind path", "/a:/b:dev,/c", []string{"/a:/b:dev", "/c"}},
		{"Spaces", "bind path", "/a::dev, suid, /c", []string{"/a::dev,suid", "/c"}},
		{"LeadingOption", "bind path", "dev,/a", []string{"dev", "/a"}},
		{"EmptyBeforeOption", "bind path", "/a::dev,,suid", []string{"/a::dev,suid"}},
		{"BlankBeforeOption", "bind path", "/a::dev, ,suid", []string{"/a::dev,suid"}},
		{"EmptyBetweenPaths", "bind path", "/a,,/b", []string{"/a", "/b"}},
		{"OtherDirective", "limit container paths", "/a,dev", []string{"/a", "dev"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.DeepEqual(t, SplitDirectiveValue(tt.directive, tt.value), tt.want)
		})
	}
}

func TestParseBindPath(t *testing.T) {
	tests := []struct {
		name        string
		entry       string
		want        BindPathEntry
		unsupported bool
		wantErr     string
	}{
		{name: "Source", entry: "/a", want: BindPathEntry{Source: "/a", Destination: "/a"}},
		{name: "Destination", entry: "/a:/b", want: BindPathEntry{Source: "/a", Destination: "/b"}},
		{name: "EmptyDestination", entry: "/a::dev", want: BindPathEntry{Source: "/a", Destination: "/a", Dev: true}},
		{name: "DevSuid", entry: "/a:/b:dev,suid", want: BindPathEntry{Source: "/a", Destination: "/b", Dev: true, Suid: true}},
		{name: "Suid", entry: "/a:/b:suid", want: BindPathEntry{Source: "/a", Destination: "/b", Suid: true}},
		{name: "EmptyOptions", entry: "/a:/b:", want: BindPathEntry{Source: "/a", Destination: "/b"}},
		{name: "UnsupportedOption", entry: "/a:/b:ro", want: BindPathEntry{Source: "/a", Destination: "/b"}, unsupported: true},
		{name: "UnsupportedAndDev", entry: "/a:/b:dev,ro", want: BindPathEntry{Source: "/a", Destination: "/b", Dev: true}, unsupported: true},
		{name: "EmptySource", entry: ":/b", wantErr: "empty source"},
		{name: "OptionWithoutDestination", entry: "/a,dev", wantErr: "mount options must follow the destination"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBindPath(tt.entry)
			switch {
			case tt.wantErr != "":
				assert.ErrorContains(t, err, tt.wantErr)
				return
			case tt.unsupported:
				assert.ErrorIs(t, err, ErrUnsupportedBindOption)
			default:
				assert.NilError(t, err)
			}
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestGetConfigBindPath(t *testing.T) {
	config, err := GetConfig(Directives{"bind path": {"/a::dev,suid", "/b,/c:/d:dev"}})
	assert.NilError(t, err)
	assert.DeepEqual(t, config.BindPath, []string{"/a::dev,suid", "/b", "/c:/d:dev"})

	// Unsupported options are ignored at runtime, so they must not stop the
	// configuration from loading.
	config, err = GetConfig(Directives{"bind path": {"/a::bogus"}})
	assert.NilError(t, err)
	assert.DeepEqual(t, config.BindPath, []string{"/a::bogus"})
}
