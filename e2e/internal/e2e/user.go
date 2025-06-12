// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/ccoveille/go-safecast"
)

// CurrentUser returns the current user account information. Use of user.Current is
// not safe with e2e tests as the user information is cached after the first call,
// so it will always return the same user information which could be wrong if
// user.Current was first called in unprivileged context and called after in a
// privileged context as it will return information of unprivileged user.
func CurrentUser(t *testing.T) *user.User {
	uid, err := safecast.ToUint32(os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	u, err := user.GetPwUID(uid)
	if err != nil {
		t.Fatalf("failed to retrieve user information")
	}
	return u
}
