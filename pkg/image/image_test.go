// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/image/unpacker"
	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"

	imageSpecs "github.com/opencontainers/image-spec/specs-go/v1"
)

// We need a busybox SIF for these tests. We used to download it each time, but we have one
// around for some e2e tests already.
const busyboxSIF = "../../e2e/testdata/busybox_" + runtime.GOARCH + ".sif"

type ownerGroupTest struct {
	name       string
	owners     []string
	privileged bool
	shouldPass bool
}

type groupTest struct {
	name       string
	groups     []string
	privileged bool
	shouldPass bool
}

// Copy the test image to a temporary location so we don't accidentally clobber the original
func copyImage(t *testing.T) string {
	f, err := os.CreateTemp("", "image-")
	if err != nil {
		t.Fatalf("cannot create temporary file: %s\n", err)
	}
	name := f.Name()
	f.Close()

	if err := fs.CopyFileAtomic(busyboxSIF, name, 0o755); err != nil {
		t.Fatalf("Could not copy test image: %v", err)
	}
	return name
}

func checkPartition(t *testing.T, reader io.Reader) error {
	extracted := "/bin/busybox"
	dir := t.TempDir()

	s := unpacker.NewSquashfs(false)
	if s.HasUnsquashfs() {
		if err := s.ExtractFiles([]string{extracted}, reader, dir); err != nil {
			return fmt.Errorf("extraction failed: %s", err)
		}
		if !fs.IsExec(filepath.Join(dir, extracted)) {
			return fmt.Errorf("%s extraction failed", extracted)
		}
	}
	return nil
}

func checkSection(_ *testing.T, reader io.Reader) error {
	dec := json.NewDecoder(reader)
	imgSpec := &imageSpecs.ImageConfig{}
	if err := dec.Decode(imgSpec); err != nil {
		return fmt.Errorf("failed to decode oci image config")
	}
	if len(imgSpec.Cmd) == 0 {
		return fmt.Errorf("no command found")
	}
	if imgSpec.Cmd[0] != "sh" {
		return fmt.Errorf("unexpected value: %s instead of sh", imgSpec.Cmd[0])
	}
	return nil
}

func TestReader(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	filename := copyImage(t)
	defer os.Remove(filename)

	for _, e := range []struct {
		fn       func(*Image, string, int) (io.Reader, error)
		fnCheck  func(*testing.T, io.Reader) error
		errCheck error
		name     string
		index    int
	}{
		{
			fn:       NewPartitionReader,
			fnCheck:  checkPartition,
			errCheck: ErrNoPartition,
			name:     RootFs,
			index:    -1,
		},
		{
			fn:       NewPartitionReader,
			fnCheck:  checkPartition,
			errCheck: ErrNoPartition,
			index:    0,
		},
		{
			fn:       NewSectionReader,
			fnCheck:  checkSection,
			errCheck: ErrNoSection,
			name:     SIFDescOCIConfigJSON,
			index:    -1,
		},
	} {
		// test with nil image parameter
		if _, err := e.fn(nil, "", -1); err == nil {
			t.Errorf("unexpected success with nil image parameter")
		}
		// test with non opened file
		if _, err := e.fn(&Image{}, "", -1); err == nil {
			t.Errorf("unexpected success with non opened file")
		}

		img, err := Init(filename, false)
		if err != nil {
			t.Fatal(err)
		}

		if img.Type != SIF {
			t.Errorf("unexpected image format: %v", img.Type)
		}
		_, err = img.GetRootFsPartition()
		if err != nil {
			t.Errorf("no root filesystem found")
		}
		// test without match criteria
		if _, err := e.fn(img, "", -1); err == nil {
			t.Errorf("unexpected success without match criteria")
		}
		// test with large index
		if _, err := e.fn(img, "", 999999); err == nil {
			t.Errorf("unexpected success with large index")
		}
		// test with unknown name
		if _, err := e.fn(img, "fakefile.name", -1); err != e.errCheck {
			t.Errorf("unexpected error with unknown name")
		}
		// test with match criteria
		if r, err := e.fn(img, e.name, e.index); err == e.errCheck {
			t.Error(err)
		} else {
			if err := e.fnCheck(t, r); err != nil {
				t.Error(err)
			}
		}
		img.File.Close()
	}
}

func TestAuthorizedPath(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tests := []struct {
		name       string
		path       []string
		shouldPass bool
	}{
		{
			name:       "empty path",
			path:       []string{""},
			shouldPass: false,
		},
		{
			name:       "invalid path",
			path:       []string{"/a/random/invalid/path"},
			shouldPass: false,
		},
		{
			name:       "valid path",
			path:       []string{"/"},
			shouldPass: true,
		},
	}

	// XXX(mem): This is what makes this test slow
	img, path := createImage(t)
	defer os.Remove(path)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auth, err := img.AuthorizedPath(test.path)
			if test.shouldPass == false && (auth == true && err == nil) {
				t.Fatal("invalid path was reported as authorized")
			}
			if test.shouldPass == true && (auth == false || err != nil) {
				if err != nil {
					t.Fatalf("valid path was reported as not authorized: %s", err)
				} else {
					t.Fatal("valid path was reported as not authorized")
				}
			}
		})
	}
}

func createImage(t *testing.T) (*Image, string) {
	// Create a temporary image
	path := copyImage(t)

	// Now load the image which will be used next for a bunch of tests
	img, err := Init(path, true)
	if err != nil {
		t.Fatal("impossible to load image for testing")
	}

	return img, path
}

func runAuthorizedOwnerTest(t *testing.T, testDescr ownerGroupTest, img *Image) {
	if testDescr.privileged == true {
		test.EnsurePrivilege(t)
	} else {
		test.DropPrivilege(t)
		defer test.ResetPrivilege(t)
	}

	auth, err := img.AuthorizedOwner(testDescr.owners)
	if testDescr.shouldPass == true && (auth == false || err != nil) {
		if err == nil {
			t.Fatalf("valid owner list reported as not authorized (%s)\n", strings.Join(testDescr.owners, ","))
		} else {
			t.Fatalf("valid test failed: %s\n", err)
		}
	}
	if testDescr.shouldPass == true && (auth == false || err != nil) {
		if err != nil {
			t.Fatalf("valid owner list was reported as not authorized: %s", err)
		} else {
			t.Fatal("valid owner list was reported as not authorized")
		}
	}
}

func TestRootAuthorizedOwner(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// Function focusing only on executing the privileged case
	test.EnsurePrivilege(t)

	tests := []ownerGroupTest{
		/* This test fails with CircleCI because of weird user management that
		   would lead to crazy code so we deactivate it for now
		{
			name:       "root",
			privileged: true,
			owners:     []string{"root"},
			shouldPass: true,
		},
		*/
		{
			name:       "invalid root",
			privileged: true,
			owners:     []string{"foobar"},
			shouldPass: false,
		},
	}

	// XXX(mem): This is what makes this test slow
	img, path := createImage(t)
	defer os.Remove(path)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runAuthorizedOwnerTest(t, tt, img)
		})
	}
}

//nolint:dupl
func TestAuthorizedOwner(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// We will create a runtime test based on the current user that assumes
	// this not a privileged test
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	// Note that we do not test the "root" case; the privileged cases are
	// tested in a separate function.
	tests := []ownerGroupTest{
		{
			name:       "empty owner list",
			privileged: false,
			owners:     []string{""},
			shouldPass: false,
		},
		{
			name:       "invalid owner list",
			privileged: false,
			owners:     []string{"2"},
			shouldPass: false,
		},
	}

	// We test with the current username, note that because we are under
	// test.DropPrivilege, this needs to be done a very specific way.
	uid := os.Getuid()
	me, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		t.Fatalf("cannot get current user name for testing purposes: %s", err)
	}
	localUser := ownerGroupTest{
		name:       "valid owner list",
		privileged: false,
		owners:     []string{me.Username},
		shouldPass: true,
	}
	tests = append(tests, localUser)

	// XXX(mem): This is what makes this test slow
	img, path := createImage(t)
	defer os.Remove(path)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runAuthorizedOwnerTest(t, test, img)
		})
	}
}

func runAuthorizedGroupTest(t *testing.T, tt groupTest, img *Image) {
	if tt.privileged == true {
		test.EnsurePrivilege(t)
	} else {
		test.DropPrivilege(t)
		defer test.ResetPrivilege(t)
	}

	auth, err := img.AuthorizedGroup(tt.groups)
	if tt.shouldPass == false && (auth == true && err == nil) {
		t.Fatal("invalid group list was reported as authorized")
	}
	if tt.shouldPass == true && (auth == false || err != nil) {
		if err != nil {
			t.Fatalf("valid group list was reported as not authorized: %s", err)
		} else {
			t.Fatal("valid group list was reported as not authorized")
		}
	}
}

func TestPrivilegedAuthorizedGroup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	test.EnsurePrivilege(t) // to make sure we create the image under the correct user

	tests := []groupTest{
		{
			name:       "root - empty group list",
			privileged: true,
			groups:     []string{""},
			shouldPass: false,
		},
		/* This case does not pass with CircleCI so we deactivate it for now
		{
			name:       "root",
			privileged: true,
			groups:     []string{"root"},
			shouldPass: true,
		},
		*/
	}

	// XXX(mem): This is what makes this test slow
	img, path := createImage(t)
	defer os.Remove(path)

	for _, tt := range tests {
		runAuthorizedGroupTest(t, tt, img)
	}
}

//nolint:dupl
func TestAuthorizedGroup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	// Note that we do not test the "root" case here, privileged cases are
	// performed in a separate function.
	tests := []groupTest{
		{
			name:       "empty group list",
			privileged: false,
			groups:     []string{""},
			shouldPass: false,
		},
		{
			name:       "invalid group list",
			privileged: false,
			groups:     []string{"-"},
			shouldPass: false,
		},
	}

	gid := os.Getgid()
	myGroup, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		t.Fatalf("cannot get group ID: %s\n", err)
	}

	validTest := groupTest{
		name:       "valid group list",
		privileged: false,
		groups:     []string{myGroup.Name},
		shouldPass: true,
	}
	tests = append(tests, validTest)

	// XXX(mem): This is what makes this test slow
	img, path := createImage(t)
	defer os.Remove(path)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runAuthorizedGroupTest(t, test, img)
		})
	}
}
