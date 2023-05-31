// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package imgbuild

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/ecl"
	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
)

var testFileContent = "Test file content\n"

type imgBuildTests struct {
	env e2e.TestEnv
}

func (c imgBuildTests) tempDir(t *testing.T, namespace string) (string, func()) {
	dn, err := fs.MakeTmpDir(c.env.TestDir, namespace+".", 0o755)
	if err != nil {
		t.Errorf("failed to create temporary directory: %#v", err)
	}

	cleanup := func() {
		if t.Failed() {
			t.Logf("Test %s failed, not removing %s", t.Name(), dn)

			return
		}

		if err := os.RemoveAll(dn); err != nil {
			t.Logf("Failed to remove %s for test %s: %#v", dn, t.Name(), err)
		}
	}

	return dn, cleanup
}

func (c imgBuildTests) buildFrom(t *testing.T) {
	e2e.EnsureORASImage(t, c.env)

	// use a trailing slash in tests for sandbox intentionally to make sure
	// `apptainer build -s /tmp/sand/ <URI>` works,
	// see https://github.com/apptainer/singularity/issues/4407
	tt := []struct {
		name         string
		dependency   string
		buildSpec    string
		requirements func(t *testing.T)
	}{
		// Disabled due to frequent download failures of the busybox tgz
		// {
		// 	name:      "BusyBox",
		// 	buildSpec: "../examples/busybox/Apptainer",
		// 	// TODO: example has arch hard coded in download URL
		//  requirements: func(t *testing.T) {
		//   require.Arch(t, "amd64")
		//  },
		// },
		{
			name:      "Debootstrap",
			buildSpec: "../examples/debian/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "debootstrap")
			},
		},
		// TODO(mem): reenable this; disabled while shub is down
		// {
		// 	name:       "ShubURI",
		// 	buildSpec:  "shub://GodloveD/busybox",
		// },
		// TODO(mem): reenable this; disabled while shub is down
		// {
		// 	name:       "ShubDefFile",
		// 	buildSpec:  "../examples/shub/Apptainer",
		// },
		{
			name:      "LibraryURI",
			buildSpec: "oras://ghcr.io/apptainer/alpine:latest",
		},
		{
			name:      "LibraryDefFile",
			buildSpec: "../examples/library/Apptainer",
		},
		{
			name:      "OrasURI",
			buildSpec: c.env.OrasTestImage,
		},
		{
			name:      "Yum CentOS7",
			buildSpec: "../examples/centos/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "yum")
				require.RPMMacro(t, "_db_backend", "bdb")
				require.RPMMacro(t, "_dbpath", "/var/lib/rpm")
				require.Arch(t, "amd64")
			},
		},
		{
			name:       "YumArm64 CentOS 7",
			dependency: "yum",
			buildSpec:  "../examples/centos-arm64/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "yum")
				require.RPMMacro(t, "_db_backend", "bdb")
				require.RPMMacro(t, "_dbpath", "/var/lib/rpm")
				require.Arch(t, "arm64")
			},
		},
		{
			name:      "Dnf AlmaLinux 9",
			buildSpec: "../examples/almalinux/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "dnf")
				require.RPMMacro(t, "_db_backend", "sqlite")
				require.RPMMacro(t, "_dbpath", "/var/lib/rpm")
				require.Arch(t, "amd64")
			},
		},
		{
			name:       "DnfArm64 AlmaLinux 9",
			dependency: "yum",
			buildSpec:  "../examples/almalinux-arm64/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "dnf")
				require.RPMMacro(t, "_db_backend", "sqlite")
				require.RPMMacro(t, "_dbpath", "/var/lib/rpm")
				require.Arch(t, "arm64")
			},
		},
		{
			name:      "Dnf Fedora 37",
			buildSpec: "../examples/fedora/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "dnf")
				require.RPMMacro(t, "_db_backend", "sqlite")
				require.RPMMacro(t, "_dbpath", "/usr/lib/sysimage/rpm")
				require.Arch(t, "amd64")
			},
		},
		{
			name:       "DnfArm64 Fedora 37",
			dependency: "yum",
			buildSpec:  "../examples/fedora-arm64/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "dnf")
				require.RPMMacro(t, "_db_backend", "sqlite")
				require.RPMMacro(t, "_dbpath", "/usr/lib/sysimage/rpm")
				require.Arch(t, "arm64")
			},
		},
		{
			name:      "Zypper",
			buildSpec: "../examples/opensuse/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "zypper")
				require.Arch(t, "amd64")
			},
		},
		{
			name:      "ZypperArm64",
			buildSpec: "../examples/opensuse-arm64/Apptainer",
			requirements: func(t *testing.T) {
				require.Command(t, "zypper")
				require.Arch(t, "arm64")
			},
		},
	}

	profiles := []e2e.Profile{e2e.RootProfile, e2e.FakerootProfile}
	for _, profile := range profiles {
		profile := profile

		t.Run(profile.String(), func(t *testing.T) {
			for _, tc := range tt {

				dn, cleanup := c.tempDir(t, "build-from")
				t.Cleanup(func() {
					if !t.Failed() {
						cleanup()
					}
				})

				imagePath := path.Join(dn, "sandbox")

				// Pass --sandbox because sandboxes take less time to
				// build by skipping the SIF creation step.
				args := []string{"--force", "--sandbox", imagePath, tc.buildSpec}

				c.env.RunApptainer(
					t,
					e2e.AsSubtest(tc.name),
					e2e.WithProfile(profile),
					e2e.WithCommand("build"),
					e2e.WithArgs(args...),
					e2e.PreRun(tc.requirements),
					e2e.PostRun(func(t *testing.T) {
						if t.Failed() {
							return
						}

						t.Cleanup(func() {
							if !t.Failed() {
								os.RemoveAll(imagePath)
							}
						})
						c.env.ImageVerify(t, imagePath, profile)
					}),
					e2e.ExpectExit(0),
				)
			}
		})
	}
}

func (c imgBuildTests) nonRootBuild(t *testing.T) {
	busyboxSIF := e2e.BusyboxSIF(t)
	tt := []struct {
		name        string
		buildSpec   string
		args        []string
		requireArch string
	}{
		{
			name:      "local sif",
			buildSpec: busyboxSIF,
		},
		{
			name:      "local sif to sandbox",
			buildSpec: busyboxSIF,
			args:      []string{"--sandbox"},
		},
		{
			name:      "library sif",
			buildSpec: "oras://ghcr.io/apptainer/busybox:1.31.1",
		},
		{
			name:      "library sif sandbox",
			buildSpec: "oras://ghcr.io/apptainer/busybox:1.31.1",
			args:      []string{"--sandbox"},
		},
		// TODO: uncomment when shub is working
		//{
		//		name:      "shub busybox",
		//		buildSpec: "shub://GodloveD/busybox",
		//},
	}

	for _, tc := range tt {
		dn, cleanup := c.tempDir(t, "non-root-build")
		t.Cleanup(func() {
			if !t.Failed() {
				cleanup()
			}
		})

		imagePath := path.Join(dn, "container")

		args := append(tc.args, imagePath, tc.buildSpec)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tc.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.PreRun(func(t *testing.T) {
				if tc.requireArch != "" {
					require.Arch(t, tc.requireArch)
				}
			}),

			e2e.PostRun(func(t *testing.T) {
				c.env.ImageVerify(t, imagePath, e2e.UserProfile)
			}),
			e2e.ExpectExit(0),
		)
	}
}

func (c imgBuildTests) buildLocalImage(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpdir, cleanup := c.tempDir(t, "build-local-image")

	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	liDefFile := e2e.PrepareDefFile(e2e.DefFileDetails{
		Bootstrap: "localimage",
		From:      c.env.ImagePath,
	})
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(liDefFile)
		}
	})

	labels := make(map[string]string)
	labels["FOO"] = "bar"
	liLabelDefFile := e2e.PrepareDefFile(e2e.DefFileDetails{
		Bootstrap: "localimage",
		From:      c.env.ImagePath,
		Labels:    labels,
	})
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(liLabelDefFile)
		}
	})

	sandboxImage := path.Join(tmpdir, "test-sandbox")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--sandbox", sandboxImage, c.env.ImagePath),
		e2e.PostRun(func(t *testing.T) {
			c.env.ImageVerify(t, sandboxImage, e2e.UserProfile)
		}),
		e2e.ExpectExit(0),
	)

	localSandboxDefFile := e2e.PrepareDefFile(e2e.DefFileDetails{
		Bootstrap: "localimage",
		From:      sandboxImage,
		Labels:    labels,
	})
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(localSandboxDefFile)
		}
	})

	tt := []struct {
		name      string
		buildSpec string
	}{
		{"SIFToSIF", c.env.ImagePath},
		{"SandboxToSIF", sandboxImage},
		{"LocalImage", liDefFile},
		{"LocalImageLabel", liLabelDefFile},
		{"LocalImageSandbox", localSandboxDefFile},
	}

	profiles := []e2e.Profile{e2e.RootProfile, e2e.FakerootProfile}
	for _, profile := range profiles {
		profile := profile

		t.Run(profile.String(), func(t *testing.T) {
			for i, tc := range tt {
				imagePath := filepath.Join(tmpdir, fmt.Sprintf("image-%d", i))
				c.env.RunApptainer(
					t,
					e2e.AsSubtest(tc.name),
					e2e.WithProfile(profile),
					e2e.WithCommand("build"),
					e2e.WithArgs(imagePath, tc.buildSpec),
					e2e.PostRun(func(t *testing.T) {
						if t.Failed() {
							return
						}
						t.Cleanup(func() {
							if !t.Failed() {
								os.RemoveAll(imagePath)
							}
						})
						c.env.ImageVerify(t, imagePath, profile)
					}),
					e2e.ExpectExit(0),
				)
			}
		})
	}
}

func (c imgBuildTests) badPath(t *testing.T) {
	dn, cleanup := c.tempDir(t, "bad-path")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	imagePath := path.Join(dn, "container")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(imagePath, "/some/dumb/path"),
		e2e.ExpectExit(255),
	)
}

func (c imgBuildTests) buildMultiStageDefinition(t *testing.T) {
	busyboxSIF := e2e.BusyboxSIF(t)
	tmpfile, err := e2e.WriteTempFile(c.env.TestDir, "testFile-", testFileContent)
	if err != nil {
		log.Fatal(err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(tmpfile)
		}
	})

	tests := []struct {
		name    string
		dfd     []e2e.DefFileDetails
		correct e2e.DefFileDetails // a bit hacky, but this allows us to check final image for correct artifacts
	}{
		// Simple copy from stage one to final stage
		{
			name: "FileCopySimple",
			dfd: []e2e.DefFileDetails{
				{
					Bootstrap: "localimage",
					From:      busyboxSIF,
					Stage:     "one",
					Files: []e2e.FilePair{
						{
							Src: tmpfile,
							Dst: "StageOne2.txt",
						},
						{
							Src: tmpfile,
							Dst: "StageOne.txt",
						},
					},
				},
				{
					Bootstrap: "localimage",
					From:      busyboxSIF,
					FilesFrom: []e2e.FileSection{
						{
							Stage: "one",
							Files: []e2e.FilePair{
								{
									Src: "StageOne2.txt",
									Dst: "StageOneCopy2.txt",
								},
								{
									Src: "StageOne.txt",
									Dst: "StageOneCopy.txt",
								},
							},
						},
					},
				},
			},
			correct: e2e.DefFileDetails{
				Files: []e2e.FilePair{
					{
						Src: tmpfile,
						Dst: "StageOneCopy2.txt",
					},
					{
						Src: tmpfile,
						Dst: "StageOneCopy.txt",
					},
				},
			},
		},
		// Complex copy of files from stage one and two to stage three, then final copy from three to final stage
		{
			name: "FileCopyComplex",
			dfd: []e2e.DefFileDetails{
				{
					Bootstrap: "localimage",
					From:      busyboxSIF,
					Stage:     "one",
					Files: []e2e.FilePair{
						{
							Src: tmpfile,
							Dst: "StageOne2.txt",
						},
						{
							Src: tmpfile,
							Dst: "StageOne.txt",
						},
					},
				},
				{
					Bootstrap: "localimage",
					From:      busyboxSIF,
					Stage:     "two",
					Files: []e2e.FilePair{
						{
							Src: tmpfile,
							Dst: "StageTwo2.txt",
						},
						{
							Src: tmpfile,
							Dst: "StageTwo.txt",
						},
					},
				},
				{
					Bootstrap: "localimage",
					From:      busyboxSIF,
					Stage:     "three",
					FilesFrom: []e2e.FileSection{
						{
							Stage: "one",
							Files: []e2e.FilePair{
								{
									Src: "StageOne2.txt",
									Dst: "StageOneCopy2.txt",
								},
								{
									Src: "StageOne.txt",
									Dst: "StageOneCopy.txt",
								},
							},
						},
						{
							Stage: "two",
							Files: []e2e.FilePair{
								{
									Src: "StageTwo2.txt",
									Dst: "StageTwoCopy2.txt",
								},
								{
									Src: "StageTwo.txt",
									Dst: "StageTwoCopy.txt",
								},
							},
						},
					},
				},
				{
					Bootstrap: "localimage",
					From:      busyboxSIF,
					FilesFrom: []e2e.FileSection{
						{
							Stage: "three",
							Files: []e2e.FilePair{
								{
									Src: "StageOneCopy2.txt",
									Dst: "StageOneCopyFinal2.txt",
								},
								{
									Src: "StageOneCopy.txt",
									Dst: "StageOneCopyFinal.txt",
								},
								{
									Src: "StageTwoCopy2.txt",
									Dst: "StageTwoCopyFinal2.txt",
								},
								{
									Src: "StageTwoCopy.txt",
									Dst: "StageTwoCopyFinal.txt",
								},
							},
						},
					},
				},
			},
			correct: e2e.DefFileDetails{
				Files: []e2e.FilePair{
					{
						Src: tmpfile,
						Dst: "StageOneCopyFinal2.txt",
					},
					{
						Src: tmpfile,
						Dst: "StageOneCopyFinal.txt",
					},
					{
						Src: tmpfile,
						Dst: "StageTwoCopyFinal2.txt",
					},
					{
						Src: tmpfile,
						Dst: "StageTwoCopyFinal.txt",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		dn, cleanup := c.tempDir(t, "multi-stage-definition")
		t.Cleanup(func() {
			if !t.Failed() {
				cleanup()
			}
		})

		imagePath := path.Join(dn, "container")

		defFile := e2e.PrepareMultiStageDefFile(tt.dfd)

		// sandboxes take less time to build
		args := []string{"--sandbox", imagePath, defFile}

		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.PostRun(func(t *testing.T) {
				t.Cleanup(func() {
					if !t.Failed() {
						os.Remove(defFile)
					}
				})

				e2e.DefinitionImageVerify(t, c.env.CmdPath, imagePath, tt.correct)
			}),
			e2e.ExpectExit(0),
		)
	}
}

//nolint:maintidx
func (c imgBuildTests) buildDefinition(t *testing.T) {
	busyboxSIF := e2e.BusyboxSIF(t)
	tmpfile, err := e2e.WriteTempFile(c.env.TestDir, "testFile-", testFileContent)
	if err != nil {
		log.Fatal(err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(tmpfile)
		}
	})

	tt := map[string]e2e.DefFileDetails{
		"Empty": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
		},
		"Help": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Help: []string{
				"help info line 1",
				"help info line 2",
				"help info line 3",
			},
		},
		"Files": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Files: []e2e.FilePair{
				{
					Src: tmpfile,
					Dst: "NewName2.txt",
				},
				{
					Src: tmpfile,
					Dst: "NewName.txt",
				},
			},
		},
		"Test": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Test: []string{
				"echo testscript line 1",
				"echo testscript line 2",
				"echo testscript line 3",
			},
		},
		"Startscript": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			StartScript: []string{
				"echo startscript line 1",
				"echo startscript line 2",
				"echo startscript line 3",
			},
		},
		"Runscript": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			RunScript: []string{
				"echo runscript line 1",
				"echo runscript line 2",
				"echo runscript line 3",
			},
		},
		"Env": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Env: []string{
				"testvar1=one",
				"testvar2=two",
				"testvar3=three",
			},
		},
		"Labels": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Labels: map[string]string{
				"customLabel1": "one",
				"customLabel2": "two",
				"customLabel3": "three",
			},
		},
		"Pre": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Pre: []string{
				filepath.Join(c.env.TestDir, "PreFile1"),
			},
		},
		"Setup": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Setup: []string{
				filepath.Join(c.env.TestDir, "SetupFile1"),
			},
		},
		"Post": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Post: []string{
				"PostFile1",
			},
		},
		"AppHelp": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Help: []string{
						"foo help info line 1",
						"foo help info line 2",
						"foo help info line 3",
					},
				},
				{
					Name: "bar",
					Help: []string{
						"bar help info line 1",
						"bar help info line 2",
						"bar help info line 3",
					},
				},
			},
		},
		"AppEnv": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Env: []string{
						"testvar1=fooOne",
						"testvar2=fooTwo",
						"testvar3=fooThree",
					},
				},
				{
					Name: "bar",
					Env: []string{
						"testvar1=barOne",
						"testvar2=barTwo",
						"testvar3=barThree",
					},
				},
			},
		},
		"AppLabels": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Labels: map[string]string{
						"customLabel1": "fooOne",
						"customLabel2": "fooTwo",
						"customLabel3": "fooThree",
					},
				},
				{
					Name: "bar",
					Labels: map[string]string{
						"customLabel1": "barOne",
						"customLabel2": "barTwo",
						"customLabel3": "barThree",
					},
				},
			},
		},
		"AppFiles": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Files: []e2e.FilePair{
						{
							Src: tmpfile,
							Dst: "FooFile2.txt",
						},
						{
							Src: tmpfile,
							Dst: "FooFile.txt",
						},
					},
				},
				{
					Name: "bar",
					Files: []e2e.FilePair{
						{
							Src: tmpfile,
							Dst: "BarFile2.txt",
						},
						{
							Src: tmpfile,
							Dst: "BarFile.txt",
						},
					},
				},
			},
		},
		"AppInstall": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Install: []string{
						"FooInstallFile1",
					},
				},
				{
					Name: "bar",
					Install: []string{
						"BarInstallFile1",
					},
				},
			},
		},
		"AppRun": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Run: []string{
						"echo foo runscript line 1",
						"echo foo runscript line 2",
						"echo foo runscript line 3",
					},
				},
				{
					Name: "bar",
					Run: []string{
						"echo bar runscript line 1",
						"echo bar runscript line 2",
						"echo bar runscript line 3",
					},
				},
			},
		},
		"AppTest": {
			Bootstrap: "localimage",
			From:      busyboxSIF,
			Apps: []e2e.AppDetail{
				{
					Name: "foo",
					Test: []string{
						"echo foo testscript line 1",
						"echo foo testscript line 2",
						"echo foo testscript line 3",
					},
				},
				{
					Name: "bar",
					Test: []string{
						"echo bar testscript line 1",
						"echo bar testscript line 2",
						"echo bar testscript line 3",
					},
				},
			},
		},
	}

	profiles := []e2e.Profile{e2e.RootProfile, e2e.FakerootProfile}
	for _, profile := range profiles {
		profile := profile

		t.Run(profile.String(), func(t *testing.T) {
			for name, dfd := range tt {
				dn, cleanup := c.tempDir(t, "build-definition")
				t.Cleanup(func() {
					if !t.Failed() {
						cleanup()
					}
				})

				imagePath := path.Join(dn, "container")

				defFile := e2e.PrepareDefFile(dfd)

				c.env.RunApptainer(
					t,
					e2e.AsSubtest(name),
					e2e.WithProfile(profile),
					e2e.WithCommand("build"),
					e2e.WithArgs("--sandbox", imagePath, defFile),
					e2e.PostRun(func(t *testing.T) {
						if t.Failed() {
							return
						}
						t.Cleanup(func() {
							if !t.Failed() {
								os.Remove(defFile)
							}
						})
						e2e.DefinitionImageVerify(t, c.env.CmdPath, imagePath, dfd)
					}),
					e2e.ExpectExit(0),
				)
			}
		})
	}
}

//nolint:maintidx
func (c imgBuildTests) buildDefinitionWithBuildArgs(t *testing.T) {
	busyboxSIF := e2e.BusyboxSIF(t)
	fileContent := "HOME=/root"
	argfile, err := e2e.WriteTempFile(c.env.TestDir, "argfile-", fileContent)
	if err != nil {
		log.Fatal(err)
	}
	t.Cleanup(func() {
		os.Remove(argfile)
	})

	tests := []struct {
		name         string
		buildArgs    []string
		buildArgFile string
		verify       []string
		deffile      string
		exit         int
		err          string
		output       string
	}{
		{
			name: "ok case single stage build",
			buildArgs: []string{
				fmt.Sprintf("IMAGE=%s", busyboxSIF),
				fmt.Sprintf("SCRIPT_PATH=%s", path.Join("testdata", "build-template", "script.sh")),
			},
			verify: []string{
				fmt.Sprintf("from: %s", busyboxSIF),
				"export AUTHOR=jason",
				"export VERSION=1",
				"Author jason",
				"Version 1",
				"testdata/build-template/script.sh",
			},
			deffile: path.Join("testdata", "build-template", "single-stage.def"),
			exit:    0,
			err:     "",
		},
		{
			name: "ko case single stage build",
			buildArgs: []string{
				fmt.Sprintf("SCRIPT_PATH=%s", path.Join("testdata", "build-template", "script.sh")),
			},
			verify:  []string{},
			deffile: path.Join("testdata", "build-template", "single-stage.def"),
			exit:    255,
			err:     "IMAGE is not defined",
		},
		{
			name: "ok case multiple stage build",
			buildArgs: []string{
				"DEVEL_IMAGE=golang:1.12.3-alpine3.9",
				"FINAL_IMAGE=alpine:3.9",
				"HOME=/root",
			},
			verify: []string{
				"from: golang:1.12.3-alpine3.9",
				"from: alpine:3.9",
				"cd /root",
			},
			deffile: path.Join("testdata", "build-template", "multiple-stage.def"),
			exit:    0,
			err:     "",
		},
		{
			name: "ok case multiple stage build with arg file",
			buildArgs: []string{
				"DEVEL_IMAGE=golang:1.12.3-alpine3.9",
				"FINAL_IMAGE=alpine:3.9",
			},
			buildArgFile: argfile,
			verify: []string{
				"from: golang:1.12.3-alpine3.9",
				"from: alpine:3.9",
				"cd /root",
			},
			deffile: path.Join("testdata", "build-template", "multiple-stage.def"),
			exit:    0,
			err:     "",
		},
		{
			name: "ko case multiple stage build",
			buildArgs: []string{
				"DEVEL_IMAGE=golang:1.12.3-alpine3.9",
				"FINAL_IMAGE=alpine:3.9",
			},
			verify:  []string{},
			deffile: path.Join("testdata", "build-template", "multiple-stage.def"),
			exit:    255,
			err:     "HOME is not defined",
		},
	}

	t.Run("build definition", func(t *testing.T) {
		for _, tt := range tests {
			dn, cleanup := c.tempDir(t, "build-definition-template")
			t.Cleanup(func() {
				if !t.Failed() {
					cleanup()
				}
			})

			imagePath := path.Join(dn, "container")
			args := []string{}

			if tt.buildArgs != nil {
				for _, arg := range tt.buildArgs {
					args = append(args, "--build-arg")
					args = append(args, arg)
				}
			}

			if tt.buildArgFile != "" {
				args = append(args, "--build-arg-file")
				args = append(args, tt.buildArgFile)
			}

			args = append(args, imagePath)
			args = append(args, tt.deffile)

			expects := []e2e.ApptainerCmdResultOp{}
			if tt.verify != nil {
				for _, v := range tt.verify {
					expects = append(expects, e2e.ExpectOutput(e2e.ContainMatch, v))
				}
			}
			expects = append(expects, e2e.ExpectOutput(e2e.UnwantedExactMatch, "{{"))
			expects = append(expects, e2e.ExpectOutput(e2e.UnwantedExactMatch, "}}"))
			c.env.RunApptainer(
				t,
				e2e.AsSubtest(tt.name),
				e2e.WithProfile(e2e.UserProfile),
				e2e.WithCommand("build"),
				e2e.WithArgs(args...),
				e2e.PostRun(func(t *testing.T) {
					if t.Failed() {
						return
					}

					if _, err := os.Stat(imagePath); errors.Is(err, os.ErrNotExist) {
						return
					}

					c.env.RunApptainer(
						t,
						e2e.AsSubtest(tt.name+" verification"),
						e2e.WithProfile(e2e.UserProfile),
						e2e.WithCommand("sif"),
						e2e.WithArgs("dump", "1", imagePath),
						e2e.ExpectExit(0,
							expects...,
						),
					)
				}),
				e2e.ExpectExit(tt.exit,
					e2e.ExpectError(e2e.ContainMatch, tt.err),
				),
			)
		}
	})
}

func (c *imgBuildTests) ensureImageIsEncrypted(t *testing.T, imgPath string) {
	sifID := "4" // Which SIF descriptor slot contains the (encrypted) rootfs
	cmdArgs := []string{"info", sifID, imgPath}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sif"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Encrypted squashfs"),
		),
	)
}

func (c *imgBuildTests) ensureImageIsGocryptfsEncrypted(t *testing.T, imgPath string) {
	sifID := "4"
	cmdArgs := []string{"info", sifID, imgPath}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sif"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Gocryptfs squashfs"),
		),
	)
}

func (c imgBuildTests) buildEncryptPemFile(t *testing.T) {
	busyboxSIF := e2e.BusyboxSIF(t)

	// Expected results for a successful command execution
	expectedExitCode := 0
	expectedStderr := ""

	// We create a temporary directory to store the image, making sure tests
	// will not pollute each other
	dn, cleanup := c.tempDir(t, "pem-encryption")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	// Generate the PEM file
	pemFile, _ := e2e.GeneratePemFiles(t, c.env.TestDir)

	// If the version of cryptsetup is not compatible with Apptainer encryption,
	// the build commands are expected to fail
	err := e2e.CheckCryptsetupVersion()
	if err != nil {
		expectedExitCode = 255
		// todo: fix the problem with catching stderr, until then we do not do a real check
		// expectedStderr = "FATAL:   While performing build: unable to encrypt filesystem at
		// /tmp/sbuild-718337349/squashfs-770818633: available cryptsetup is not supported"
		expectedStderr = ""
	}

	// First with the command line argument
	imgPath1 := filepath.Join(dn, "encrypted_cmdline_option.sif")
	cmdArgs := []string{"--encrypt", "--pem-path", pemFile, imgPath1, busyboxSIF}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			expectedExitCode,
			e2e.ExpectError(e2e.ContainMatch, expectedStderr),
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsEncrypted(t, imgPath1)
	}

	// Second with the environment variable
	pemEnvVar := fmt.Sprintf("%s=%s", "APPTAINER_ENCRYPTION_PEM_PATH", pemFile)
	imgPath2 := filepath.Join(dn, "encrypted_env_var.sif")
	cmdArgs = []string{"--encrypt", imgPath2, busyboxSIF}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), pemEnvVar)),
		e2e.ExpectExit(
			expectedExitCode,
			e2e.ExpectError(e2e.ContainMatch, expectedStderr),
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsEncrypted(t, imgPath2)
	}
}

// buildEncryptPassphrase is exercising the build command for encrypted containers
// while using a passphrase. Note that it covers both the normal case and when the
// version of cryptsetup available is not compliant.
func (c imgBuildTests) buildEncryptPassphrase(t *testing.T) {
	busyboxSIF := e2e.BusyboxSIF(t)

	// Expected results for a successful command execution
	expectedExitCode := 0
	expectedStderr := ""

	// We create a temporary directory to store the image, making sure tests
	// will not pollute each other
	dn, cleanup := c.tempDir(t, "passphrase-encryption")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	// If the version of cryptsetup is not compatible with Apptainer encryption,
	// the build commands are expected to fail
	err := e2e.CheckCryptsetupVersion()
	if err != nil {
		expectedExitCode = 255
		expectedStderr = ": installed version of cryptsetup is not supported, >=2.0.0 required"
	}

	// First with the command line argument, only using --passphrase
	passphraseInput := []e2e.ApptainerConsoleOp{
		e2e.ConsoleSendLine(e2e.Passphrase),
	}
	cmdlineTestImgPath := filepath.Join(dn, "encrypted_cmdline_option.sif")
	// The image is deleted during cleanup of the temporary directory
	cmdArgs := []string{"--passphrase", cmdlineTestImgPath, busyboxSIF}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("passphrase flag"),
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.ConsoleRun(passphraseInput...),
		e2e.ExpectExit(
			expectedExitCode,
			e2e.ExpectError(e2e.ContainMatch, expectedStderr),
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsEncrypted(t, cmdlineTestImgPath)
	}

	// With the command line argument, using --encrypt and --passphrase
	cmdlineTest2ImgPath := filepath.Join(dn, "encrypted_cmdline2_option.sif")
	cmdArgs = []string{"--encrypt", "--passphrase", cmdlineTest2ImgPath, busyboxSIF}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("encrypt and passphrase flags"),
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.ConsoleRun(passphraseInput...),
		e2e.ExpectExit(
			expectedExitCode,
			e2e.ExpectError(e2e.ContainMatch, expectedStderr),
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsEncrypted(t, cmdlineTest2ImgPath)
	}

	// With the environment variable
	passphraseEnvVar := fmt.Sprintf("%s=%s", "APPTAINER_ENCRYPTION_PASSPHRASE", e2e.Passphrase)
	envvarImgPath := filepath.Join(dn, "encrypted_env_var.sif")
	cmdArgs = []string{"--encrypt", envvarImgPath, busyboxSIF}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("passphrase env var"),
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(
			expectedExitCode,
			e2e.ExpectError(e2e.ContainMatch, expectedStderr),
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsEncrypted(t, envvarImgPath)
	}

	// Finally a test that must fail: try to specify the passphrase on the command line
	dummyImgPath := filepath.Join(dn, "dummy_encrypted_env_var.sif")
	cmdArgs = []string{"--encrypt", "--passphrase", e2e.Passphrase, dummyImgPath, busyboxSIF}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("passphrase on cmdline"),
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
		e2e.ExpectExit(
			1,
			e2e.ExpectError(e2e.RegexMatch, `^Error for command \"build\": accepts 2 arg\(s\), received 3`),
		),
	)
}

func (c imgBuildTests) buildUpdateSandbox(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	const badSandbox = "/bad/sandbox/path"

	testDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "build-sandbox-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			e2e.Privileged(cleanup)
		}
	})

	tests := []struct {
		name     string
		args     []string
		exitCode int
	}{
		{
			name:     "Sandbox",
			args:     []string{"--force", "--sandbox", testDir, c.env.ImagePath},
			exitCode: 0,
		},
		{
			name:     "UpdateWithoutSandboxFlag",
			args:     []string{"--update", testDir, c.env.ImagePath},
			exitCode: 255,
		},
		{
			name:     "UpdateWithBadSandboxpPath",
			args:     []string{"--update", "--sandbox", badSandbox, c.env.ImagePath},
			exitCode: 255,
		},
		{
			name:     "UpdateWithFileAsSandbox",
			args:     []string{"--update", "--sandbox", c.env.ImagePath, c.env.ImagePath},
			exitCode: 255,
		},
		{
			name:     "UpdateSandbox",
			args:     []string{"--update", "--sandbox", testDir, c.env.ImagePath},
			exitCode: 0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exitCode),
		)
	}
}

// buildWithFingerprint checks that we correctly verify a source image fingerprint when specified
func (c imgBuildTests) buildWithFingerprint(t *testing.T) {
	tmpDir, remove := e2e.MakeTempDir(t, "", "imgbuild-fingerprint-", "")
	t.Cleanup(func() {
		c.env.KeyringDir = ""
		remove(t)
	})

	pgpDir, _ := e2e.MakeKeysDir(t, tmpDir)
	c.env.KeyringDir = pgpDir
	invalidFingerPrint := "0000000000000000000000000000000000000000"
	singleSigned := filepath.Join(tmpDir, "singleSigned.sif")
	doubleSigned := filepath.Join(tmpDir, "doubleSigned.sif")
	unsigned := filepath.Join(tmpDir, "unsigned.sif")
	output := filepath.Join(tmpDir, "output.sif")

	// Prepare the test source images
	prep := []struct {
		name       string
		command    string
		args       []string
		consoleOps []e2e.ApptainerConsoleOp
	}{
		{
			name:    "import key1 local",
			command: "key import",
			args:    []string{"testdata/ecl-pgpkeys/key1.asc"},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
		},
		{
			name:    "import key2 local",
			command: "key import",
			args:    []string{"testdata/ecl-pgpkeys/key2.asc"},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
		},
		{
			name:    "build single signed source image",
			command: "build",
			args:    []string{singleSigned, e2e.BusyboxSIF(t)},
		},
		{
			name:    "build double signed source image",
			command: "build",
			args:    []string{doubleSigned, singleSigned},
		},
		{
			name:    "build unsigned source image",
			command: "build",
			args:    []string{unsigned, singleSigned},
		},
		{
			name:    "sign single signed image with key1",
			command: "sign",
			args:    []string{"-k", "0", singleSigned},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
		},
		{
			name:    "sign double signed image with key1",
			command: "sign",
			args:    []string{"-k", "0", doubleSigned},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
		},
		{
			name:    "sign double signed image with key2",
			command: "sign",
			args:    []string{"-k", "1", doubleSigned},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
		},
	}

	for _, tt := range prep {
		cmdOps := []e2e.ApptainerCmdOp{
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0),
		}
		if tt.consoleOps != nil {
			cmdOps = append(cmdOps, e2e.ConsoleRun(tt.consoleOps...))
		}
		c.env.RunApptainer(t, cmdOps...)
	}

	// Test builds with "Fingerprint:" headers
	tests := []struct {
		name       string
		definition string
		exit       int
		wantErr    string
	}{
		{
			name:       "build single signed one fingerprint",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s\n", singleSigned, ecl.KeyMap["key1"]),
			exit:       0,
		},
		{
			name:       "build single signed two fingerprints",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s,%s\n", singleSigned, ecl.KeyMap["key1"], ecl.KeyMap["key2"]),
			exit:       255,
			wantErr:    "image not signed by required entities",
		},
		{
			name:       "build single signed one wrong fingerprint",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s\n", singleSigned, invalidFingerPrint),
			exit:       255,
			wantErr:    "image not signed by required entities",
		},
		{
			name:       "build single signed two fingerprints one wrong",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s,%s\n", singleSigned, invalidFingerPrint, ecl.KeyMap["key2"]),
			exit:       255,
			wantErr:    "image not signed by required entities",
		},
		{
			name:       "build double signed one fingerprint",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s\n", doubleSigned, ecl.KeyMap["key1"]),
			exit:       0,
		},
		{
			name:       "build double signed two fingerprints",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s,%s\n", doubleSigned, ecl.KeyMap["key1"], ecl.KeyMap["key2"]),
			exit:       0,
		},
		{
			name:       "build double signed one wrong fingerprint",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s\n", doubleSigned, invalidFingerPrint),
			exit:       255,
			wantErr:    "image not signed by required entities",
		},
		{
			name:       "build double signed two fingerprints one wrong",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s,%s\n", doubleSigned, invalidFingerPrint, ecl.KeyMap["key2"]),
			exit:       255,
			wantErr:    "image not signed by required entities",
		},
		{
			name:       "build unsigned one fingerprint",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s\n", unsigned, ecl.KeyMap["key1"]),
			exit:       255,
			wantErr:    "signature not found",
		},
		{
			name:       "build unsigned two fingerprints",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints: %s,%s\n", unsigned, ecl.KeyMap["key1"], ecl.KeyMap["key2"]),
			exit:       255,
			wantErr:    "signature not found",
		},
		{
			name:       "build unsigned empty fingerprints",
			definition: fmt.Sprintf("Bootstrap: localimage\nFrom: %s\nFingerprints:\n", unsigned),
			exit:       0,
		},
	}

	for _, tt := range tests {
		defFile, err := e2e.WriteTempFile(c.env.TestDir, "testFile-", tt.definition)
		if err != nil {
			log.Fatal(err)
		}
		t.Cleanup(func() {
			if !t.Failed() {
				os.Remove(defFile)
			}
		})
		c.env.RunApptainer(t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs("-F", output, defFile),
			e2e.ExpectExit(tt.exit,
				e2e.ExpectError(e2e.ContainMatch, tt.wantErr),
			),
		)
	}
}

// buildBindMount checks that we can bind host files/directories during build.
func (c imgBuildTests) buildBindMount(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpdir, cleanup := c.tempDir(t, "build-local-image")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	dir, _ := e2e.MakeTempDir(t, tmpdir, "mount", "")

	canaryFile := filepath.Join(dir, "canary")
	if err := fs.Touch(canaryFile); err != nil {
		t.Fatalf("while touching %s: %v", canaryFile, err)
	}

	tests := []struct {
		name        string
		buildOption []string
		buildPost   []string
		buildTest   []string
		exit        int
	}{
		{
			name: "Bind test dir to /mnt",
			buildOption: []string{
				"--bind", dir + ":/mnt",
			},
			buildPost: []string{
				"cat /mnt/canary",
			},
			buildTest: []string{
				"cat /mnt/canary",
			},
			exit: 0,
		},
		{
			name: "Bind test dir to multiple directory",
			buildOption: []string{
				"--bind", dir + ":/mnt",
				"--bind", dir + ":/opt",
			},
			buildPost: []string{
				"cat /mnt/canary",
				"cat /opt/canary",
			},
			buildTest: []string{
				"cat /mnt/canary",
				"cat /opt/canary",
			},
			exit: 0,
		},
		{
			name: "Bind test dir to /mnt read-only",
			buildOption: []string{
				"--bind", dir + ":/mnt:ro",
			},
			buildPost: []string{
				"mkdir /mnt/should_fail",
			},
			exit: 255,
		},
		{
			name: "Bind test dir to non-existent image directory",
			buildOption: []string{
				"--bind", dir + ":/fake/dir",
			},
			buildPost: []string{
				"cat /mnt/canary",
			},
			exit: 255,
		},
		{
			name: "Mount test dir to /mnt",
			buildOption: []string{
				"--mount", "type=bind,source=" + dir + ",destination=/mnt",
			},
			buildPost: []string{
				"cat /mnt/canary",
			},
			buildTest: []string{
				"cat /mnt/canary",
			},
			exit: 0,
		},
		{
			name: "Mount test dir to multiple directory",
			buildOption: []string{
				"--mount", "type=bind,source=" + dir + ",destination=/mnt",
				"--mount", "type=bind,source=" + dir + ",destination=/opt",
			},
			buildPost: []string{
				"cat /mnt/canary",
				"cat /opt/canary",
			},
			buildTest: []string{
				"cat /mnt/canary",
				"cat /opt/canary",
			},
			exit: 0,
		},
		{
			name: "Mount test dir to /mnt read-only",
			buildOption: []string{
				"--mount", "type=bind,source=" + dir + ",destination=/mnt,ro",
			},
			buildPost: []string{
				"mkdir /mnt/should_fail",
			},
			exit: 255,
		},
		{
			name: "Mount test dir to non-existent image directory",
			buildOption: []string{
				"--mount", "type=bind,source=" + dir + ",destination=/fake/dir",
			},
			buildPost: []string{
				"cat /mnt/canary",
			},
			exit: 255,
		},
	}

	sandboxImage := filepath.Join(tmpdir, "build-sandbox")

	definition := fmt.Sprintf("Bootstrap: localimage\nFrom: %s", c.env.ImagePath)

	for _, tt := range tests {
		rawDef := definition
		if len(tt.buildPost) > 0 {
			rawDef += fmt.Sprintf("\n%%post\n\t%s", strings.Join(tt.buildPost, "\n"))
		}
		if len(tt.buildTest) > 0 {
			rawDef += fmt.Sprintf("\n%%test\n\t%s", strings.Join(tt.buildTest, "\n"))
		}
		defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(rawDef))

		args := tt.buildOption
		args = append(args, "-F", "--sandbox", sandboxImage, defFile)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs(args...),
			e2e.PostRun(func(t *testing.T) {
				os.Remove(defFile)
			}),
			e2e.ExpectExit(tt.exit),
		)
	}
}

func (c imgBuildTests) buildLibraryHost(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpdir, cleanup := c.tempDir(t, "build-libraryhost-test")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	// Library hostname in the From URI
	// The hostname is invalid, and we should get an error to that effect.
	definition := "Bootstrap: library\nLibrary: https://library.example.com\nFrom: test/test/test:latest\n"

	defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(definition))
	imagePath := filepath.Join(tmpdir, "image-libaryhost")
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("-F", imagePath, defFile),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(defFile)
		}),
		e2e.ExpectExit(255,
			e2e.ExpectError(e2e.ContainMatch, "no such host"),
		),
	)
}

// testWritableTmpfs checks that we can run the build using a writeable tmpfs in the %test step
func (c imgBuildTests) testWritableTmpfs(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tmpdir, cleanup := c.tempDir(t, "build-writabletmpfs-test")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	// Definition will attempt to touch a file in /var/test during %test.
	// This would fail without a writable tmpfs.
	definition := fmt.Sprintf("Bootstrap: localimage\nFrom: %s\n%%test\ntouch /var/test\n", c.env.ImagePath)

	defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(definition))
	imagePath := filepath.Join(tmpdir, "image-writabletmpfs")
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("-F", "--writable-tmpfs", imagePath, defFile),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(defFile)
		}),
		e2e.ExpectExit(0),
	)
}

// testBuildEnvironmentVariables tests the environment variables exposed by the build system when executing
// definition sections. This includes APPTAINER_ROOTFS, APPTAINER_ENVIRONMENT, APPTAINER_LABELS and their
// SINGULARITY_ prefixed counterparts.
func (c imgBuildTests) testBuildEnvironmentVariables(t *testing.T) {
	const definition = `Bootstrap: docker
From: alpine:latest

%setup
touch $APPTAINER_ROOTFS/rootfs-file
touch $SINGULARITY_ROOTFS/legacy-rootfs-file

%post
echo 'test-label value' >> $APPTAINER_LABELS
echo 'legacy-test-label legacy-value' >> $SINGULARITY_LABELS
echo 'export TEST_ENV=value' >> $APPTAINER_ENVIRONMENT
echo 'export LEGACY_TEST_ENV=legacy-value' >> $SINGULARITY_ENVIRONMENT
`

	tmpdir, cleanup := c.tempDir(t, "build-environment-test")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	defFile, err := e2e.WriteTempFile(tmpdir, "testFile-", definition)
	if err != nil {
		log.Fatal(err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(defFile)
		}
	})

	imagePath := filepath.Join(tmpdir, "image-build-environment")
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("-F", imagePath, defFile),
		e2e.ExpectExit(0),
	)

	// Check to ensure the *_ROOTFS commands in the definition succeeded in file creation.
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(imagePath, "ls", "/rootfs-file", "/legacy-rootfs-file"),
		e2e.ExpectExit(0),
	)

	// Check to ensure the *_ENVIRONMENT commands in the definition successfully set the env.
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(imagePath, "env"),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "TEST_ENV=value"),
			e2e.ExpectOutput(e2e.ContainMatch, "LEGACY_TEST_ENV=legacy-value"),
		),
	)

	// Check to ensure the *_LABELS commands in the definition successfully set labels.
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("inspect"),
		e2e.WithArgs("-l", imagePath),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "test-label: value"),
			e2e.ExpectOutput(e2e.ContainMatch, "legacy-test-label: legacy-value"),
		),
	)
}

func (c *imgBuildTests) testContainerBuildUnderFakerootModes(t *testing.T) {
	e2e.EnsureDebianImage(t, c.env)

	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "test-container-build-under-fakeroot-modes-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})

	// Make the DebianImagePath available for Bootstrap: localimage
	sif := c.env.DebianImagePath
	basesif := filepath.Base(sif)
	err := os.Symlink(sif, basesif)
	if err != nil {
		t.Fatalf("while symlinking %s to %s: %v", sif, basesif, err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			os.Remove(basesif)
		}
	})

	// running under the mode 1, 1a (--with-suid) (https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", fmt.Sprintf("%s/openssh-mode1a.sif", tmpDir), "testdata/unprivileged_build.def"),
		e2e.ExpectExit(0),
	)

	// running under the mode 1, 1b (--without-suid) (https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--userns", fmt.Sprintf("%s/openssh-mode1b.sif", tmpDir), "testdata/unprivileged_build.def"),
		e2e.ExpectExit(0),
	)

	// running under the mode 2(https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserNamespaceProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--userns", "--ignore-subuid", "--ignore-fakeroot-command", fmt.Sprintf("%s/openssh-mode2a.sif", tmpDir), "testdata/unprivileged_build.def"),
		e2e.ExpectExit(255), // because chown will fail
	)

	// running under the mode 2(https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserNamespaceProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--userns", "--ignore-subuid", "--ignore-fakeroot-command", fmt.Sprintf("%s/openssh-mode2b.sif", tmpDir), "testdata/unprivileged_build_2.def"),
		e2e.ExpectExit(0),
	)

	// running under the mode 3(https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--userns", "--ignore-subuid", fmt.Sprintf("%s/openssh-mode3.sif", tmpDir), "testdata/unprivileged_build.def"),
		e2e.ExpectExit(0),
	)

	// running under the mode 4(https://apptainer.org/docs/user/main/fakeroot.html)
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.FakerootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--ignore-userns", "--ignore-subuid", fmt.Sprintf("%s/openssh-mode4.sif", tmpDir), "testdata/unprivileged_build_4.def"),
		e2e.ExpectExit(0),
	)
}

func (c *imgBuildTests) testSIFHeaderAndExecute(t *testing.T) {
	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "test-SIF-header-fields-", "")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup(t)
		}
	})

	path := fmt.Sprintf("%s/img.sif", tmpDir)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(path, "docker://busybox"),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sif"),
		e2e.WithArgs("header", path),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "#!/usr/bin/env run-singularity"),
			e2e.ExpectOutput(e2e.RegexMatch, ".*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}.*"),
		))

	output, err := exec.Command(path, "echo", "-n", "hello").Output()
	if err != nil {
		t.Errorf("could not execute the sif image: %s, error: %s", path, err.Error())
		t.Fail()
	}
	if string(output) != "hello" {
		t.Errorf("the output is not expected, expected: hello, actual: %s", string(output))
		t.Fail()
	}
}

// Check that test and runscript that specify a custom #! use it as the interpreter.
func (c imgBuildTests) buildCustomShebang(t *testing.T) {
	tmpdir, cleanup := c.tempDir(t, "build-shebang-test")
	t.Cleanup(func() {
		if !t.Failed() {
			cleanup()
		}
	})

	definition := `Bootstrap: localimage
From: %s

%%test
#!/bin/busybox sh
cat /proc/$$/cmdline

%%runscript
#!/bin/busybox sh
cat /proc/$$/cmdline`

	definition = fmt.Sprintf(definition, e2e.BusyboxSIF(t))

	defFile := e2e.RawDefFile(t, tmpdir, strings.NewReader(definition))
	imagePath := filepath.Join(tmpdir, "image-shebang")

	// build time %test script
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("-F", imagePath, defFile),
		e2e.PostRun(func(t *testing.T) {
			os.Remove(defFile)
		}),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "/bin/busybox"),
		),
	)
	// runtime %runscript
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("run"),
		e2e.WithArgs(imagePath),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "/bin/busybox"),
		),
	)
	// runtime %test script
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("test"),
		e2e.WithArgs(imagePath),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(e2e.ContainMatch, "/bin/busybox"),
		),
	)
}

func (c imgBuildTests) testGocryptfsSIFBuild(t *testing.T) {
	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "test-gocryptfs-sif-build-", "")
	defer cleanup(t)

	// Setting pem file via cli
	imgcli := fmt.Sprintf("%s/imgcli.sif", tmpDir)
	pubKey, _ := e2e.GeneratePemFiles(t, tmpDir)
	expectedExitCode := 0
	busybox := e2e.BusyboxSIF(t)
	cmdArgs := []string{"--pem-path", pubKey, imgcli, busybox}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			expectedExitCode,
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsGocryptfsEncrypted(t, imgcli)
	}

	// Setting pem file via environment variable
	imgenv := fmt.Sprintf("%s/imgenv.sif", tmpDir)
	cmdArgs = []string{"--pem-path", pubKey, imgenv, busybox}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv([]string{fmt.Sprintf("APPTAINER_ENCRYPTION_PEM_PATH=%s", pubKey)}),
		e2e.ExpectExit(
			expectedExitCode,
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsGocryptfsEncrypted(t, imgenv)
	}

	// Setting passphrase via cli
	imgcliPass := fmt.Sprintf("%s/imgcliPass.sif", tmpDir)
	cmdArgs = []string{"--passphrase", imgcliPass, busybox}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithStdin(strings.NewReader("1234\n")),
		e2e.ExpectExit(
			expectedExitCode,
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsGocryptfsEncrypted(t, imgcliPass)
	}

	// Setting passphrase via environment variable
	imgenvPass := fmt.Sprintf("%s/imgenvPass.sif", tmpDir)
	cmdArgs = []string{"--encrypt", imgenvPass, busybox}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithEnv([]string{"APPTAINER_ENCRYPTION_PASSPHRASE=1234"}),
		e2e.ExpectExit(
			expectedExitCode,
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsGocryptfsEncrypted(t, imgenvPass)
	}

	// Setting passphrase via both cli and environment variable
	imgclienvPass := fmt.Sprintf("%s/imgclienvPass.sif", tmpDir)
	cmdArgs = []string{"--passphrase", "--encrypt", imgclienvPass, busybox}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(cmdArgs...),
		e2e.WithStdin(strings.NewReader("1234\n")),
		e2e.WithEnv([]string{"APPTAINER_ENCRYPTION_PASSPHRASE=1234"}),
		e2e.ExpectExit(
			expectedExitCode,
		),
	)
	// If the command was supposed to succeed, we check the image
	if expectedExitCode == 0 {
		c.ensureImageIsGocryptfsEncrypted(t, imgclienvPass)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := imgBuildTests{
		env: env,
	}

	return testhelper.Tests{
		"bad path":                               c.badPath,                              // try to build from a non existent path
		"build encrypt with PEM file":            c.buildEncryptPemFile,                  // build encrypted images with certificate
		"build encrypted with passphrase":        c.buildEncryptPassphrase,               // build encrypted images with passphrase
		"definition":                             c.buildDefinition,                      // builds from definition template
		"from local image":                       c.buildLocalImage,                      // build and image from an existing image
		"from":                                   c.buildFrom,                            // builds from definition file and URI
		"multistage":                             c.buildMultiStageDefinition,            // multistage build from definition templates
		"non-root build":                         c.nonRootBuild,                         // build sifs from non-root
		"build and update sandbox":               c.buildUpdateSandbox,                   // build/update sandbox
		"fingerprint check":                      c.buildWithFingerprint,                 // definition file includes fingerprint check
		"build with bind mount":                  c.buildBindMount,                       // build image with bind mount
		"library host":                           c.buildLibraryHost,                     // build image with hostname in library URI
		"customShebang":                          c.buildCustomShebang,                   // build image with custom #! in %test and %runscript
		"test with writable tmpfs":               c.testWritableTmpfs,                    // build image, using writable tmpfs in the test step
		"test build system environment":          c.testBuildEnvironmentVariables,        // build image with build system environment variables set in definition
		"test build under fakeroot modes":        c.testContainerBuildUnderFakerootModes, // build image under different fakeroot modes
		"issue 3848":                             c.issue3848,                            // https://github.com/apptainer/singularity/issues/3848
		"issue 4203":                             c.issue4203,                            // https://github.com/apptainer/singularity/issues/4203
		"issue 4407":                             c.issue4407,                            // https://github.com/apptainer/singularity/issues/4407
		"issue 4583":                             c.issue4583,                            // https://github.com/apptainer/singularity/issues/4583
		"issue 4820":                             c.issue4820,                            // https://github.com/apptainer/singularity/issues/4820
		"issue 4837":                             c.issue4837,                            // https://github.com/apptainer/singularity/issues/4837
		"issue 4967":                             c.issue4967,                            // https://github.com/apptainer/singularity/issues/4967
		"issue 4969":                             c.issue4969,                            // https://github.com/apptainer/singularity/issues/4969
		"issue 5166":                             c.issue5166,                            // https://github.com/apptainer/singularity/issues/5166
		"issue 5250":                             c.issue5250,                            // https://github.com/apptainer/singularity/issues/5250
		"issue 5315":                             c.issue5315,                            // https://github.com/apptainer/singularity/issues/5315
		"issue 5435":                             c.issue5435,                            // https://github.com/apptainer/singularity/issues/5435
		"issue 5668":                             c.issue5668,                            // https://github.com/apptainer/singularity/issues/5435
		"issue 5690":                             c.issue5690,                            // https://github.com/apptainer/singularity/issues/5690
		"test sif header and execute image":      c.testSIFHeaderAndExecute,              // https://github.com/apptainer/apptainer/issues/211
		"build sif image using gocryptfs":        c.testGocryptfsSIFBuild,                // https://github.com/apptainer/apptainer/issues/484
		"definition build with template support": c.buildDefinitionWithBuildArgs,         // builds from definition with build args (build arg file) support
	}
}
