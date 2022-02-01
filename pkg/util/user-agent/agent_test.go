// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package useragent

import (
	"regexp"
	"testing"
)

func TestApptainerVersion(t *testing.T) {
	InitValue("apptainer", "v0.1.0-30-g67692d50f-dirty")

	re := regexp.MustCompile(`Apptainer/v[[:digit:]]+(.[[:digit:]]+){2} \(Linux [[:alnum:]]+\) Go/[[:digit:]]+(.[[:digit:]]+){1,2}`)
	if !re.MatchString(Value()) {
		t.Fatalf("user agent did not match regexp")
	}
}
