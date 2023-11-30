// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"fmt"
	"syscall"

	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
)

// DriverFeature defines a feature type that a driver is supporting.
type DriverFeature uint16

const (
	// SquashFeature means the driver handles squashfs image mounts.
	SquashFeature DriverFeature = 1 << iota
	// Ext3Feature means the driver handles ext3fs image mounts.
	Ext3Feature
	// GocryptFeature means the driver handles gocryptfs image mounts.
	GocryptFeature
	// OverlayFeature means the driver handle overlay mount.
	OverlayFeature
	// FuseFeature means the driver uses FUSE as its base.
	FuseFeature
)

// ImageFeature means the driver handles any of the image mount types
const ImageFeature = SquashFeature | Ext3Feature | GocryptFeature

// MountFunc defines mount function prototype
type MountFunc func(source string, target string, filesystem string, flags uintptr, data string) error

// MountParams defines parameters passed to driver interface
// while mounting images.
type MountParams struct {
	Source           string   // image source
	Target           string   // image target mount point
	Filesystem       string   // image filesystem type
	Flags            uintptr  // mount flags
	Offset           uint64   // offset where start filesystem
	Size             uint64   // size of image filesystem
	Key              []byte   // filesystem decryption key
	FSOptions        []string // filesystem mount options
	DontElevatePrivs bool     // omit cmd.SysProcAttr, currently only used by gocryptfs
}

// DriverParams defines parameters passed to driver interface
// while starting it.
type DriverParams struct {
	SessionPath string         // session driver image path
	UsernsFd    int            // user namespace file descriptor
	FuseFd      int            // fuse file descriptor
	Config      *config.Common // common engine configuration
}

// Driver defines the image driver interface to register.
type Driver interface {
	// Mount is called each time an engine mount an image
	Mount(*MountParams, MountFunc) error
	// Start the driver for initialization.
	Start(*DriverParams, int) error
	// Stop the driver related to given mount target for cleanup.
	Stop(string) error
	// Check if any of the image driver processes matches the given
	// pid that exited with the given status and return an error if
	// one of them does, or nil if they do not.
	Stopped(int, syscall.WaitStatus) error
	// Features Feature returns supported features.
	Features() DriverFeature
}

// drivers holds all registered image drivers
var drivers = make(map[string]Driver)

// RegisterDriver registers an image driver by name.
func RegisterDriver(name string, driver Driver) error {
	if name == "" {
		return fmt.Errorf("empty name")
	} else if _, ok := drivers[name]; ok {
		return fmt.Errorf("%s is already registered", name)
	} else if driver == nil {
		return fmt.Errorf("nil driver")
	}
	drivers[name] = driver
	return nil
}

// GetDriver returns the named image driver interface.
func GetDriver(name string) Driver {
	return drivers[name]
}
