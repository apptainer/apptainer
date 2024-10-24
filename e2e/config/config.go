// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
)

type configTests struct {
	env                  e2e.TestEnv
	sifImage             string
	encryptedImage       string
	encryptedUnprivImage string
	squashfsImage        string
	ext3Image            string
	ext3OverlayImage     string
	sandboxImage         string
	pemPublic            string
	pemPrivate           string
}

// prepImages creates containers covering all image formats to test the
// `allow container xxx` directives.
func (c *configTests) prepImages(t *testing.T) (cleanup func(t *testing.T)) {
	require.MkfsExt3(t)
	require.Command(t, "truncate")
	require.Command(t, "mksquashfs")

	tmpDir, cleanup := e2e.MakeTempDir(t, "", "config-", "CONFIG")

	// An unencrypted SIF
	e2e.EnsureImage(t, c.env)
	c.sifImage = c.env.ImagePath

	// Encrypted SIFs, privileged and unprivileged
	c.pemPublic, c.pemPrivate = e2e.GeneratePemFiles(t, tmpDir)
	c.encryptedImage = filepath.Join(tmpDir, "encrypted.sif")
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("PrepareEncryptedSIF"),
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--encrypt", "--pem-path", c.pemPublic, c.encryptedImage, c.sifImage),
		e2e.ExpectExit(0),
	)
	c.encryptedUnprivImage = filepath.Join(tmpDir, "encryptedUnpriv.sif")
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("PrepareEncryptedUnprivSIF"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--encrypt", "--pem-path", c.pemPublic, c.encryptedUnprivImage, c.sifImage),
		e2e.ExpectExit(0),
	)

	// A sandbox directory
	c.sandboxImage = filepath.Join(tmpDir, "sandbox")
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("PrepareSandbox"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("-s", c.sandboxImage, c.sifImage),
		e2e.ExpectExit(0),
	)

	// A bare ext3 image
	t.Run("PrepareExt3", func(t *testing.T) {
		c.ext3Image = filepath.Join(tmpDir, "ext3.img")
		cmd := exec.Command("truncate", "-s", "16M", c.ext3Image)
		if out, err := cmd.CombinedOutput(); err != nil {
			defer cleanup(t)
			t.Fatalf("Error creating blank ext3 image: %v: %s", err, out)
		}
		cmd = exec.Command("mkfs.ext3", "-d", c.sandboxImage, c.ext3Image)
		if out, err := cmd.CombinedOutput(); err != nil {
			defer cleanup(t)
			t.Fatalf("Error creating populated ext3 image: %v: %s", err, out)
		}
	})

	// A bare squashfs image
	t.Run("PrepareSquashfs", func(t *testing.T) {
		c.squashfsImage = filepath.Join(tmpDir, "squashfs.img")
		cmd := exec.Command("mksquashfs", c.sandboxImage, c.squashfsImage)
		if out, err := cmd.CombinedOutput(); err != nil {
			defer cleanup(t)
			t.Fatalf("Error creating squashfs image: %v: %s", err, out)
		}
	})

	// An ext3 overlay embedded in a SIF
	c.ext3OverlayImage = filepath.Join(tmpDir, "ext3Overlay.img")
	if err := fs.CopyFile(c.sifImage, c.ext3OverlayImage, 0o755); err != nil {
		t.Fatalf("Could not copy test image file: %v", err)
	}
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("PrepareExt3Overlay"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("overlay"),
		e2e.WithArgs("create", c.ext3OverlayImage),
		e2e.ExpectExit(0),
	)

	return cleanup
}

//nolint:maintidx
func (c configTests) configGlobal(t *testing.T) {
	cleanup := c.prepImages(t)
	defer cleanup(t)

	e2e.SetDirective(t, c.env, "allow setuid-mount extfs", "yes")
	defer e2e.ResetDirective(t, c.env, "allow setuid-mount extfs")

	u := e2e.UserProfile.HostUser(t)
	g, err := user.GetGrGID(u.GID)
	if err != nil {
		t.Fatalf("could not retrieve user group information: %s", err)
	}

	tests := []struct {
		name              string
		argv              []string
		profile           e2e.Profile
		addRequirementsFn func(*testing.T)
		cwd               string
		directive         string
		directiveValue    string
		exit              int
		resultOp          e2e.ApptainerCmdResultOp
	}{
		{
			name: "AllowSetuid",
			argv: []string{c.env.ImagePath, "true"},
			// We are testing if we fall back to user namespace without `--userns`
			// so we need to use the UserProfile, and check separately if userns
			// support is possible.
			profile:           e2e.UserProfile,
			addRequirementsFn: require.UserNamespace,
			directive:         "allow setuid",
			directiveValue:    "no",
			exit:              0,
		},
		{
			name: "MaxLoopDevices",
			argv: []string{c.env.ImagePath, "true"},
			// RootProfile makes sure that kernel squashfs is used
			profile:        e2e.RootProfile,
			directive:      "max loop devices",
			directiveValue: "0",
			exit:           255,
		},
		{
			name:           "AllowPidNsNo",
			argv:           []string{"--pid", "--no-init", c.env.ImagePath, "/bin/sh", "-c", "echo $$"},
			profile:        e2e.UserProfile,
			directive:      "allow pid ns",
			directiveValue: "no",
			exit:           0,
			resultOp:       e2e.ExpectOutput(e2e.UnwantedExactMatch, "1"),
		},
		{
			name:           "AllowPidNsYes",
			argv:           []string{"--pid", "--no-init", c.env.ImagePath, "/bin/sh", "-c", "echo $$"},
			profile:        e2e.UserProfile,
			directive:      "allow pid ns",
			directiveValue: "yes",
			exit:           0,
			resultOp:       e2e.ExpectOutput(e2e.ExactMatch, "1"),
		},
		{
			name:           "AllowUtsNsNo",
			argv:           []string{"--uts", "--hostname", "foo", c.env.ImagePath, "hostname"},
			profile:        e2e.UserProfile,
			directive:      "allow uts ns",
			directiveValue: "no",
			exit:           0,
			resultOp:       e2e.ExpectOutput(e2e.UnwantedExactMatch, "foo"),
		},
		{
			name:           "AllowUtsNsYes",
			argv:           []string{"--uts", "--hostname", "foo", c.env.ImagePath, "hostname"},
			profile:        e2e.UserProfile,
			directive:      "allow uts ns",
			directiveValue: "yes",
			exit:           0,
			resultOp:       e2e.ExpectOutput(e2e.ExactMatch, "foo"),
		},
		{
			name:           "ConfigPasswdNo",
			argv:           []string{c.env.ImagePath, "grep", "/etc/passwd.*- tmpfs", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "config passwd",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "ConfigPasswdYes",
			argv:           []string{c.env.ImagePath, "grep", "/etc/passwd.*- tmpfs", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "config passwd",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "ConfigGroupNo",
			argv:           []string{c.env.ImagePath, "grep", "/etc/group.*- tmpfs", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "config group",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "ConfigGroupYes",
			argv:           []string{c.env.ImagePath, "grep", "/etc/group.*- tmpfs", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "config group",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "ConfigResolvConfNo",
			argv:           []string{c.env.ImagePath, "grep", "/etc/resolv.conf.*- tmpfs", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "config resolv_conf",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "ConfigResolvConfYes",
			argv:           []string{c.env.ImagePath, "grep", "/etc/resolv.conf.*- tmpfs", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "config resolv_conf",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "MountProcNo",
			argv:           []string{c.env.ImagePath, "test", "-d", "/proc/self"},
			profile:        e2e.UserProfile,
			directive:      "mount proc",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "MountProcYes",
			argv:           []string{c.env.ImagePath, "test", "-d", "/proc/self"},
			profile:        e2e.UserProfile,
			directive:      "mount proc",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "MountSysNo",
			argv:           []string{c.env.ImagePath, "test", "-d", "/sys/kernel"},
			profile:        e2e.UserProfile,
			directive:      "mount sys",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "MountSysYes",
			argv:           []string{c.env.ImagePath, "test", "-d", "/sys/kernel"},
			profile:        e2e.UserProfile,
			directive:      "mount sys",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "MountDevNo",
			argv:           []string{c.env.ImagePath, "test", "-d", "/dev/pts"},
			profile:        e2e.UserProfile,
			directive:      "mount dev",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "MountDevMinimal",
			argv:           []string{c.env.ImagePath, "test", "-b", "/dev/loop0"},
			profile:        e2e.UserProfile,
			directive:      "mount dev",
			directiveValue: "minimal",
			exit:           1,
		},
		{
			name:           "MountDevYes",
			argv:           []string{c.env.ImagePath, "test", "-b", "/dev/loop0"},
			profile:        e2e.UserProfile,
			directive:      "mount dev",
			directiveValue: "yes",
			exit:           0,
		},
		// just test 'mount devpts = no' as yes depends of kernel version
		{
			name:           "MountDevPtsNo",
			argv:           []string{"-C", c.env.ImagePath, "test", "-d", "/dev/pts"},
			profile:        e2e.UserProfile,
			directive:      "mount devpts",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "MountHomeNo",
			argv:           []string{c.env.ImagePath, "test", "-d", u.Dir},
			profile:        e2e.UserProfile,
			cwd:            "/",
			directive:      "mount home",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "MountHomeYes",
			argv:           []string{c.env.ImagePath, "test", "-d", u.Dir},
			profile:        e2e.UserProfile,
			cwd:            "/",
			directive:      "mount home",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "MountTmpNo",
			argv:           []string{c.env.ImagePath, "test", "-d", c.env.TestDir},
			profile:        e2e.UserProfile,
			directive:      "mount tmp",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "MountTmpYes",
			argv:           []string{c.env.ImagePath, "test", "-d", c.env.TestDir},
			profile:        e2e.UserProfile,
			directive:      "mount tmp",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "BindPathPasswd",
			argv:           []string{c.env.ImagePath, "test", "-f", "/passwd"},
			profile:        e2e.UserProfile,
			directive:      "bind path",
			directiveValue: "/etc/passwd:/passwd",
			exit:           0,
		},
		{
			name:           "UserBindControlNo",
			argv:           []string{"--bind", "/etc/passwd:/passwd", c.env.ImagePath, "test", "-f", "/passwd"},
			profile:        e2e.UserProfile,
			directive:      "user bind control",
			directiveValue: "no",
			exit:           1,
		},
		{
			name:           "UserBindControlYes",
			argv:           []string{"--bind", "/etc/passwd:/passwd", c.env.ImagePath, "test", "-f", "/passwd"},
			profile:        e2e.UserProfile,
			directive:      "user bind control",
			directiveValue: "yes",
			exit:           0,
		},
		// overlay may or not be available, just test with no
		//nolint:dupword
		{
			name:           "EnableOverlayNo",
			argv:           []string{c.env.ImagePath, "grep", "\\- overlay overlay", "/proc/self/mountinfo"},
			profile:        e2e.UserProfile,
			directive:      "enable overlay",
			directiveValue: "no",
			exit:           1,
		},
		// test image is owned by root:root
		{
			name:           "LimitContainerOwnersUser",
			argv:           []string{c.env.ImagePath, "true"},
			profile:        e2e.UserProfile,
			directive:      "limit container owners",
			directiveValue: u.Name,
			exit:           255,
		},
		{
			name:           "LimitContainerOwnersUserAndRoot",
			argv:           []string{c.env.ImagePath, "true"},
			profile:        e2e.UserProfile,
			directive:      "limit container owners",
			directiveValue: u.Name + ", root",
			exit:           0,
		},
		{
			name:           "LimitContainerGroupsUser",
			argv:           []string{c.env.ImagePath, "true"},
			profile:        e2e.UserProfile,
			directive:      "limit container groups",
			directiveValue: g.Name,
			exit:           255,
		},
		{
			name:           "LimitContainerGroupsUserAndRoot",
			argv:           []string{c.env.ImagePath, "true"},
			profile:        e2e.UserProfile,
			directive:      "limit container groups",
			directiveValue: g.Name + ", root",
			exit:           0,
		},
		{
			name:           "LimitContainerPathsProc",
			argv:           []string{c.env.ImagePath, "true"},
			profile:        e2e.UserProfile,
			directive:      "limit container paths",
			directiveValue: "/proc",
			exit:           255,
		},
		{
			name:           "LimitContainerPathsTestdir",
			argv:           []string{c.env.ImagePath, "true"},
			profile:        e2e.UserProfile,
			directive:      "limit container paths",
			directiveValue: c.env.TestDir,
			exit:           0,
		},
		{
			name:           "AllowContainerSifNo",
			argv:           []string{c.sifImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container sif",
			directiveValue: "no",
			exit:           255,
		},
		{
			name:           "AllowContainerSifYes",
			argv:           []string{c.sifImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container sif",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "AllowContainerEncryptedNo",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container encrypted",
			directiveValue: "no",
			exit:           255,
		},
		{
			name:           "AllowContainerEncryptedYes",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container encrypted",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "AllowContainerSquashfsNo",
			argv:           []string{c.squashfsImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container squashfs",
			directiveValue: "no",
			exit:           255,
		},
		{
			name:           "AllowContainerSquashfsYes",
			argv:           []string{c.squashfsImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container squashfs",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "AllowContainerExfs3No",
			argv:           []string{c.ext3Image, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container extfs",
			directiveValue: "no",
			exit:           255,
		},
		{
			name:           "AllowContainerExtfsYes",
			argv:           []string{c.ext3Image, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container extfs",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "AllowContainerDirNo",
			argv:           []string{c.sandboxImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container dir",
			directiveValue: "no",
			exit:           255,
		},
		{
			name:           "AllowContainerDirYes",
			argv:           []string{c.sandboxImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow container dir",
			directiveValue: "yes",
			exit:           0,
		},
		// NOTE: the "allow setuid-mount" tests have to stay after the
		// "allow container" tests because they will be left in their
		// default settings which can interfere with "allow container" tests.
		{
			name:           "AllowSetuidMountEncryptedNo",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount encrypted",
			directiveValue: "no",
			exit:           255,
		},
		{
			name:           "AllowSetuidMountEncryptedNoUnpriv",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedUnprivImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount encrypted",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountEncryptedNoUserns",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedUnprivImage, "true"},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount encrypted",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountEncryptedYes",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount encrypted",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountEncryptedYesUnpriv",
			argv:           []string{"--pem-path", c.pemPrivate, c.encryptedUnprivImage, "true"},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount encrypted",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsNo",
			argv:           []string{c.squashfsImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsNoSif",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsNoBind",
			argv:           []string{"-B", c.squashfsImage + ":/sqsh:image-src=/", c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsNoUserns",
			argv:           []string{c.squashfsImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsNoUsernsSif",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsNoUsernsBind",
			argv:           []string{"-B", c.squashfsImage + ":/sqsh:image-src=/", c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsIflimitedNone",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "iflimited",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountSquashfsIflimitedPaths",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "limit container paths",
			directiveValue: c.env.TestDir,
			exit:           1,
		},
		{
			name:           "AllowSetuidMountSquashfsIflimitedOwners",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "limit container owners",
			directiveValue: u.Name + ", root",
			exit:           1,
		},
		{
			name:           "AllowSetuidMountSquashfsIflimitedGroups",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "limit container groups",
			directiveValue: g.Name + ", root",
			exit:           1,
		},
		// the ECL check with iflimited is done in the ecl e2e tests
		{
			name:           "AllowSetuidMountSquashfsYes",
			argv:           []string{c.squashfsImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "yes",
			exit:           1,
		},
		{
			name:           "AllowSetuidMountSquashfsYesSif",
			argv:           []string{c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "yes",
			exit:           1,
		},
		{
			name:           "AllowSetuidMountSquashfsYesBind",
			argv:           []string{"-B", c.squashfsImage + ":/sqsh:image-src=/", c.sifImage, "sh", "-c", e2e.Findsquash},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount squashfs",
			directiveValue: "yes",
			exit:           1,
		},
		{
			name:           "AllowSetuidMountExtfsNo",
			argv:           []string{c.ext3Image, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountExtfsNoSif",
			argv:           []string{c.ext3OverlayImage, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountExtfsNoBind",
			argv:           []string{"-B", c.ext3Image + ":/ext3:image-src=/", c.sifImage, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountExtfsNoUserns",
			argv:           []string{c.ext3Image, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountExtfsNoUsernsSif",
			argv:           []string{c.ext3OverlayImage, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountExtfsNoUsernsBind",
			argv:           []string{"-B", c.ext3Image + ":/ext3:image-src=/", c.sifImage, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserNamespaceProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "no",
			exit:           0,
		},
		{
			name:           "AllowSetuidMountExtfsYes",
			argv:           []string{c.ext3Image, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "yes",
			exit:           1,
		},
		{
			name:           "AllowSetuidMountExtfsYesSif",
			argv:           []string{c.ext3OverlayImage, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "yes",
			exit:           1,
		},
		{
			name:           "AllowSetuidMountExtfsYesBind",
			argv:           []string{"-B", c.ext3Image + ":/ext3:image-src=/", c.sifImage, "sh", "-c", e2e.Findfuse2fs},
			profile:        e2e.UserProfile,
			directive:      "allow setuid-mount extfs",
			directiveValue: "yes",
			exit:           1,
		},
		{
			name:           "SystemdCgroupsYes",
			argv:           []string{"--apply-cgroups", "testdata/cgroups/pids_limit.toml", c.sandboxImage, "true"},
			profile:        e2e.RootProfile,
			directive:      "systemd cgroups",
			directiveValue: "yes",
			exit:           0,
		},
		{
			name:           "SystemdCgroupNo",
			argv:           []string{"--apply-cgroups", "testdata/cgroups/pids_limit.toml", c.sandboxImage, "true"},
			profile:        e2e.RootProfile,
			directive:      "systemd cgroups",
			directiveValue: "no",
			exit:           0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithDir(tt.cwd),
			e2e.PreRun(func(t *testing.T) {
				if tt.addRequirementsFn != nil {
					tt.addRequirementsFn(t)
				}
				e2e.SetDirective(t, c.env, tt.directive, tt.directiveValue)
			}),
			e2e.PostRun(func(t *testing.T) {
				e2e.ResetDirective(t, c.env, tt.directive)
			}),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit, tt.resultOp),
		)
	}
}

// Tests that require combinations of directives to be set
func (c configTests) configGlobalCombination(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	setDirectives := func(t *testing.T, directives map[string]string) {
		for k, v := range directives {
			e2e.SetDirective(t, c.env, k, v)
		}
	}
	resetDirectives := func(t *testing.T, directives map[string]string) {
		for k := range directives {
			e2e.ResetDirective(t, c.env, k)
		}
	}

	u := e2e.UserProfile.HostUser(t)
	g, err := user.GetGrGID(u.GID)
	if err != nil {
		t.Fatalf("could not retrieve user group information: %s", err)
	}

	tests := []struct {
		name              string
		argv              []string
		profile           e2e.Profile
		addRequirementsFn func(*testing.T)
		cwd               string
		directives        map[string]string
		exit              int
		resultOp          e2e.ApptainerCmdResultOp
	}{
		{
			name:    "AllowNetUsersNobody",
			argv:    []string{"--net", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net users": "nobody",
			},
			exit: 255,
		},
		{
			name:    "AllowNetUsersUser",
			argv:    []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net users": u.Name,
			},
			exit: 255,
		},
		{
			name:    "AllowNetUsersUID",
			argv:    []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net users": fmt.Sprintf("%d", u.UID),
			},
			exit: 255,
		},
		{
			name:              "AllowNetUsersUserOK",
			addRequirementsFn: e2e.Privileged(require.Network),
			argv:              []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile:           e2e.UserProfile,
			directives: map[string]string{
				"allow net users":    u.Name,
				"allow net networks": "bridge",
			},
			exit: 0,
		},
		{
			name:              "AllowNetUsersUIDOK",
			addRequirementsFn: e2e.Privileged(require.Network),
			argv:              []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile:           e2e.UserProfile,
			directives: map[string]string{
				"allow net users":    fmt.Sprintf("%d", u.UID),
				"allow net networks": "bridge",
			},
			exit: 0,
		},
		{
			name:    "AllowNetGroupsNobody",
			argv:    []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net groups": "nobody",
			},
			exit: 255,
		},
		{
			name:    "AllowNetGroupsGroup",
			argv:    []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net groups": g.Name,
			},
			exit: 255,
		},
		{
			name:    "AllowNetGroupsGID",
			argv:    []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net groups": fmt.Sprintf("%d", g.GID),
			},
			exit: 255,
		},
		{
			name:              "AllowNetGroupsGroupOK",
			addRequirementsFn: e2e.Privileged(require.Network),
			argv:              []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile:           e2e.UserProfile,
			directives: map[string]string{
				"allow net groups":   g.Name,
				"allow net networks": "bridge",
			},
			exit: 0,
		},
		{
			name:              "AllowNetGroupsGIDOK",
			addRequirementsFn: e2e.Privileged(require.Network),
			argv:              []string{"--net", "--network", "bridge", c.env.ImagePath, "true"},
			profile:           e2e.UserProfile,
			directives: map[string]string{
				"allow net groups":   fmt.Sprintf("%d", g.GID),
				"allow net networks": "bridge",
			},
			exit: 0,
		},
		{
			name:              "AllowNetNetworksMultiMulti",
			addRequirementsFn: e2e.Privileged(require.Network),
			// Two networks allowed, asking for both
			argv:    []string{"--net", "--network", "bridge,ptp", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net users":    u.Name,
				"allow net networks": "bridge,ptp",
			},
			exit: 0,
		},
		{
			// Two networks allowed, asking for one
			name:              "AllowNetNetworksMultiOne",
			addRequirementsFn: e2e.Privileged(require.Network),
			argv:              []string{"--net", "--network", "ptp", c.env.ImagePath, "true"},
			profile:           e2e.UserProfile,
			directives: map[string]string{
				"allow net users":    u.Name,
				"allow net networks": "bridge,ptp",
			},
			exit: 0,
		},
		{
			// One network allowed, but asking for two
			name:    "AllowNetNetworksOneMulti",
			argv:    []string{"--net", "--network", "bridge,ptp", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net users":    u.Name,
				"allow net networks": "bridge",
			},
			exit: 255,
		},
		{
			// No networks allowed, asking for two
			name:    "AllowNetNetworksNoneMulti",
			argv:    []string{"--net", "--network", "bridge,ptp", c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"allow net users": u.Name,
			},
			exit: 255,
		},
		{
			name:    "EnableOverlayNoUnderlayNo",
			argv:    []string{"--bind", "/etc/passwd:/passwd", c.env.ImagePath, "test", "-f", "/passwd"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"enable overlay":  "no",
				"enable underlay": "no",
			},

			exit: 255,
		},
		{
			name:    "EnableUnderlayYes",
			argv:    []string{"--bind", "/etc/passwd:/passwd", c.env.ImagePath, "sh", "-c", "test -f /passwd && mount"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"enable overlay":  "no",
				"enable underlay": "yes",
			},
			resultOp: e2e.ExpectOutput(e2e.ContainMatch, "on / type tmpfs"),
			exit:     0,
		},
		{
			name:    "EnableUnderlayPreferred",
			argv:    []string{"--bind", "/etc/passwd:/passwd", c.env.ImagePath, "sh", "-c", "test -f /passwd && mount"},
			profile: e2e.UserProfile,
			directives: map[string]string{
				"enable underlay": "preferred",
			},
			resultOp: e2e.ExpectOutput(e2e.ContainMatch, "on / type tmpfs"),
			exit:     0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithDir(tt.cwd),
			e2e.PreRun(func(t *testing.T) {
				if tt.addRequirementsFn != nil {
					tt.addRequirementsFn(t)
				}
				setDirectives(t, tt.directives)
			}),
			e2e.PostRun(func(t *testing.T) {
				resetDirectives(t, tt.directives)
			}),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit, tt.resultOp),
		)
	}
}

func (c configTests) configFile(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name    string
		argv    []string
		profile e2e.Profile
		conf    string
		exit    int
	}{
		{
			name:    "MaxLoopDevicesKO",
			argv:    []string{c.env.ImagePath, "true"},
			profile: e2e.RootProfile,
			conf:    "max loop devices = 0\n",
			exit:    255,
		},
		{
			name:    "MaxLoopDevicesOK",
			argv:    []string{c.env.ImagePath, "true"},
			profile: e2e.RootProfile,
			conf:    "max loop devices = 128\n",
			exit:    0,
		},
		{
			name:    "UserForbidden",
			argv:    []string{c.env.ImagePath, "true"},
			profile: e2e.UserProfile,
			conf:    "max loop devices = 128\n",
			exit:    255,
		},
	}

	// Create a temp testfile
	f, err := fs.MakeTmpFile(c.env.TestDir, "config-", 0o644)
	if err != nil {
		t.Fatal(err)
	}
	configFile := f.Name()
	defer os.Remove(configFile)
	f.Close()

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithGlobalOptions("--config", configFile),
			e2e.WithProfile(tt.profile),
			e2e.PreRun(func(t *testing.T) {
				if err := os.WriteFile(configFile, []byte(tt.conf), 0o644); err != nil {
					t.Errorf("could not write configuration file %s: %s", configFile, err)
				}
			}),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := configTests{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"config file":               c.configFile,                  // test --config file option
		"config global":             np(c.configGlobal),            // test various global configuration
		"config global combination": np(c.configGlobalCombination), // test various global configuration with combination
	}
}
