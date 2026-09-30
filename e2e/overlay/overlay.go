// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

package overlay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

func (c ctx) testOverlayCreate(t *testing.T) {
	require.Filesystem(t, "overlay")
	require.MkfsExt3(t)
	e2e.EnsureImage(t, c.env)
	busyboxSIF := e2e.BusyboxSIF(t)

	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "overlay", "")
	defer cleanup(t)

	pgpDir, _ := e2e.MakeKeysDir(t, tmpDir)
	c.env.KeyringDir = pgpDir

	sifSignedImage := filepath.Join(tmpDir, "signed.sif")
	sifImage := filepath.Join(tmpDir, "unsigned.sif")
	ext3SparseImage := filepath.Join(tmpDir, "image.sparse.ext3")
	ext3Image := filepath.Join(tmpDir, "image.ext3")
	ext3DirImage := filepath.Join(tmpDir, "imagedir.ext3")

	// signed SIF image
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(sifSignedImage, busyboxSIF),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("key import"),
		e2e.WithArgs("testdata/ecl-pgpkeys/key1.asc"),
		e2e.ConsoleRun(e2e.ConsoleSendLine("e2e")),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sign"),
		e2e.WithArgs("-k", "0", sifSignedImage),
		e2e.ConsoleRun(e2e.ConsoleSendLine("e2e")),
		e2e.ExpectExit(0),
	)

	// unsigned SIF image
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs(sifImage, busyboxSIF),
		e2e.ExpectExit(0),
	)

	type test struct {
		name    string
		profile e2e.Profile
		command string
		args    []string
		exit    int
	}

	tests := []test{
		{
			name:    "create ext3 overlay with small size",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", "--size", "1", ext3Image},
			exit:    255,
		},
		{
			name:    "create ext3 sparse overlay image",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", "--size", "128", "--sparse", ext3SparseImage},
			exit:    0,
		},
		{
			name:    "create ext3 overlay image",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", "--size", "128", ext3Image},
			exit:    0,
		},
		{
			name:    "check ext3 overlay size",
			profile: e2e.UserProfile,
			command: "exec",
			args:    []string{"-B", ext3Image + ":/mnt/image", c.env.ImagePath, "/bin/sh", "-c", "[ $(stat -c %s /mnt/image) = 134217728 ] || false"},
			exit:    0,
		},
		{
			name:    "create ext3 overlay with an existing image",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", ext3Image},
			exit:    255,
		},
		{
			name:    "create ext3 overlay with dir",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", "--create-dir", "/usr/local/testing", ext3DirImage},
			exit:    0,
		},
		{
			name:    "check overlay dir permissions",
			profile: e2e.UserProfile,
			command: "exec",
			args:    []string{"-o", ext3DirImage, c.env.ImagePath, "mkdir", "/usr/local/testing/perms"},
			exit:    0,
		},
		{
			name:    "create ext3 overlay image in unsigned SIF",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", sifImage},
			exit:    0,
		},
		{
			name:    "create ext3 overlay image in SIF with an existing overlay",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", sifImage},
			exit:    255,
		},
		{
			name:    "create ext3 overlay image in signed SIF",
			profile: e2e.UserProfile,
			command: "overlay",
			args:    []string{"create", sifSignedImage},
			exit:    255,
		},
	}

	err := e2e.CheckCryptsetupVersion()
	if err == nil {
		// encrypted SIF image
		passphraseEnvVar := fmt.Sprintf("%s=%s", "APPTAINER_ENCRYPTION_PASSPHRASE", e2e.Passphrase)

		sifEncryptedImage := filepath.Join(tmpDir, "encrypted.sif")

		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("build"),
			e2e.WithArgs("--encrypt", sifEncryptedImage, busyboxSIF),
			e2e.WithEnv(append(os.Environ(), passphraseEnvVar)),
			e2e.ExpectExit(0),
		)

		tests = append(tests, test{
			name:    "create ext3 overlay image in encrypted SIF",
			profile: e2e.RootProfile,
			command: "overlay",
			args:    []string{"create", sifEncryptedImage},
			exit:    255,
		})
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

func (c ctx) createOverlayBuild(t *testing.T, baseImage string, tmpDir string) string {
	definition := e2e.PrepareDefFile(e2e.DefFileDetails{
		Bootstrap: "localimage",
		From:      baseImage,
		Post:      []string{"echo overlay-marker > /overlay-marker"},
	})
	t.Cleanup(func() {
		if !t.Failed() {
			if err := os.Remove(definition); err != nil {
				t.Logf("failed to remove definition file %s: %v", definition, err)
			}
		}
	})

	overlayImage := filepath.Join(tmpDir, "overlay.sif")
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserNamespaceProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--overlay", overlayImage, definition),
		e2e.ExpectExit(0),
	)

	return overlayImage
}

func (c ctx) testBuildOverlay(t *testing.T) {
	baseImage := e2e.BusyboxSIF(t)
	tmpDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "build-overlay", "")
	t.Cleanup(func() {
		cleanup(t)
	})

	overlayImage := c.createOverlayBuild(t, baseImage, tmpDir)

	overlaySquashfs := filepath.Join(tmpDir, "overlay.squashfs")

	// Can't use RunApptainer here because need to redirect stdout
	cmd := exec.Command("/bin/sh", "-c", "apptainer sif dump 4 "+overlayImage+"> "+overlaySquashfs)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	t.Log(cmd.Args)
	if err != nil {
		t.Fatalf("Failed dumping squashfs partition from %s\n%s: %s", baseImage, err, string(out))
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--overlay", overlaySquashfs, baseImage, "test", "-f", "/overlay-marker"),
		e2e.ExpectExit(0),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"create":        c.testOverlayCreate,
		"build overlay": c.testBuildOverlay,
	}
}
