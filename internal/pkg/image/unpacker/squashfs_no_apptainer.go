// Copyright (c) 2021, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build !apptainer_engine
// +build !apptainer_engine

package unpacker

func init() {
	cmdFunc = unsquashfsCmd
}
