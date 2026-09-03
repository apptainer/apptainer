// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestNVCLIEnvToFlags(t *testing.T) {
	tests := []struct {
		name      string
		env       []string
		wantFlags []string
		wantErr   bool
	}{
		{
			name: "defaults",
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
				"--utility",
			},
			wantErr: false,
		},
		{
			name: "device",
			env: []string{
				"NVIDIA_VISIBLE_DEVICES=all",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--device=all",
				"--compute",
				"--utility",
			},
			wantErr: false,
		},
		{
			name: "mig-config",
			env: []string{
				"NVIDIA_MIG_CONFIG_DEVICES=all",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--mig-config=all",
				"--compute",
				"--utility",
			},
			wantErr: false,
		},
		{
			name: "mig-monitor",
			env: []string{
				"NVIDIA_MIG_MONITOR_DEVICES=all",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--mig-monitor=all",
				"--compute",
				"--utility",
			},
			wantErr: false,
		},
		{
			name: "compute-only",
			env: []string{
				"NVIDIA_DRIVER_CAPABILITIES=compute",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
			},
			wantErr: false,
		},
		{
			name: "all-caps",
			env: []string{
				"NVIDIA_DRIVER_CAPABILITIES=compute,compat32,graphics,utility,video,display,ngx",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
				"--compat32",
				"--graphics",
				"--utility",
				"--video",
				"--display",
				"--ngx",
			},
			wantErr: false,
		},
		{
			name: "every-cap",
			env: []string{
				"NVIDIA_DRIVER_CAPABILITIES=all",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
				"--compat32",
				"--graphics",
				"--utility",
				"--video",
				"--display",
				"--ngx",
			},
			wantErr: false,
		},
		{
			name: "invalid-caps",
			env: []string{
				"NVIDIA_DRIVER_CAPABILITIES=notacap",
			},
			wantErr: true,
		},
		{
			name: "single-require",
			env: []string{
				"NVIDIA_REQUIRE_CUDA=cuda>=9.0",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
				"--utility",
				"--require=cuda>=9.0",
			},
			wantErr: false,
		},
		{
			name: "multi-require",
			env: []string{
				"NVIDIA_REQUIRE_BRAND=brand=GRID",
				"NVIDIA_REQUIRE_CUDA=cuda>=9.0",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
				"--utility",
				"--require=brand=GRID",
				"--require=cuda>=9.0",
			},
			wantErr: false,
		},
		{
			name: "disable-require",
			env: []string{
				"NVIDIA_REQUIRE_BRAND=brand=GRID",
				"NVIDIA_REQUIRE_CUDA=cuda>=9.0",
				"NVIDIA_DISABLE_REQUIRE=1",
			},
			wantFlags: []string{
				"--no-cgroups",
				"--compute",
				"--utility",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFlags, err := NVCLIEnvToFlags(tt.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("NVCLIEnvToFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			sort.Strings(gotFlags)
			sort.Strings(tt.wantFlags)
			if !reflect.DeepEqual(gotFlags, tt.wantFlags) {
				t.Errorf("NVCLIEnvToFlags() = %v, want %v", gotFlags, tt.wantFlags)
			}
		})
	}
}

// TestNVCLILibraries checks that the libraries are those the list command of
// nvidia-container-cli prints, one per line, and that the command is run in
// user mode unless root.
func TestNVCLILibraries(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	cli := filepath.Join(dir, "nvidia-container-cli")
	script := "#!/bin/sh\necho \"$*\" > " + argsFile + "\necho /usr/lib/libtestgpu.so.1.2.3\necho /usr/lib/libtestgpu-ml.so.1.2.3\n"
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatalf("Could not create file: %v", err)
	}

	got, err := nvcliLibraries(cli)
	if err != nil {
		t.Fatalf("nvcliLibraries() error = %v", err)
	}
	want := []string{"/usr/lib/libtestgpu.so.1.2.3", "/usr/lib/libtestgpu-ml.so.1.2.3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nvcliLibraries() = %q, expected %q", got, want)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("The command was not run: %v", err)
	}
	wantArgs := "list --libraries\n"
	if os.Geteuid() != 0 {
		wantArgs = "--user list --libraries\n"
	}
	if string(args) != wantArgs {
		t.Errorf("nvidia-container-cli was run with %q, expected %q", args, wantArgs)
	}
	if _, err := nvcliLibraries(filepath.Join(dir, "absent")); err == nil {
		t.Error("nvcliLibraries() did not fail for an absent command")
	}
}
