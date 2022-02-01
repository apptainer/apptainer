// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	ocibundle "github.com/apptainer/apptainer/pkg/ocibundle/sif"
)

// OciMount mount a SIF image to create an OCI bundle
func OciMount(image string, bundle string) error {
	d, err := ocibundle.FromSif(image, bundle, true)
	if err != nil {
		return err
	}
	return d.Create(nil)
}

// OciUmount umount SIF and delete OCI bundle
func OciUmount(bundle string) error {
	d, err := ocibundle.FromSif("", bundle, true)
	if err != nil {
		return err
	}
	return d.Delete()
}
