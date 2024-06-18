// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package server

import (
	"os"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/util/fs"

	args "github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer/rpc"
	server "github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer/rpc/server"
	ociargs "github.com/apptainer/apptainer/internal/pkg/runtime/engine/oci/rpc"
	"github.com/apptainer/apptainer/internal/pkg/util/mainthread"
)

// Methods is a receiver type.
type Methods struct {
	*server.Methods
}

// MkdirAll performs a mkdir with the specified arguments.
func (t *Methods) MkdirAll(arguments *args.MkdirArgs, _ *int) (err error) {
	mainthread.Execute(func() {
		oldmask := syscall.Umask(0)
		err = os.MkdirAll(arguments.Path, arguments.Perm)
		syscall.Umask(oldmask)
	})
	return err
}

// Touch performs a touch with the specified arguments.
func (t *Methods) Touch(arguments *ociargs.TouchArgs, _ *int) (err error) {
	return fs.Touch(arguments.Path)
}
