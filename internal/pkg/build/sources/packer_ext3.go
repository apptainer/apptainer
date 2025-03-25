// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sources

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/apptainer/apptainer/pkg/build/types"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/archive"
	"github.com/apptainer/apptainer/pkg/util/loop"
	"golang.org/x/sys/unix"
)

// Ext3Packer holds the locations of where to back from and to, as well as image offset info
type Ext3Packer struct {
	srcfile string
	b       *types.Bundle
	img     *image.Image
}

// Pack puts relevant objects in a Bundle!
func (p *Ext3Packer) Pack(context.Context) (*types.Bundle, error) {
	err := unpackExt3(p.b, p.img)
	if err != nil {
		sylog.Errorf("while unpacking ext3 image: %v", err)
		return nil, err
	}

	return p.b, nil
}

// unpackExt3 mounts the ext3 image using a loop device and then copies its contents to the bundle
func unpackExt3(b *types.Bundle, img *image.Image) error {
	info := &unix.LoopInfo64{
		Offset:    img.Partitions[0].Offset,
		Sizelimit: img.Partitions[0].Size,
		Flags:     unix.LO_FLAGS_AUTOCLEAR,
	}

	var number int
	maxLoopDev, err := loop.GetMaxLoopDevices()
	if err != nil {
		return err
	}
	loopdev := &loop.Device{
		MaxLoopDevices: maxLoopDev,
		Info:           info,
	}

	if err := loopdev.AttachFromFile(img.File, os.O_RDONLY, &number); err != nil {
		return fmt.Errorf("while attaching image to loop device: %v", err)
	}

	tmpmnt, err := os.MkdirTemp(b.TmpDir, "mnt")
	if err != nil {
		return fmt.Errorf("while making tmp mount point: %v", err)
	}

	path := fmt.Sprintf("/dev/loop%d", number)
	sylog.Debugf("Mounting loop device %s to %s\n", path, tmpmnt)
	err = syscall.Mount(path, tmpmnt, "ext3", syscall.MS_NOSUID|syscall.MS_RDONLY|syscall.MS_NODEV, "errors=remount-ro")
	if err != nil {
		return fmt.Errorf("while mounting image: %v", err)
	}
	defer syscall.Unmount(tmpmnt, 0)

	// copy filesystem into bundle rootfs
	sylog.Debugf("Copying filesystem from %s to %s in Bundle\n", tmpmnt, b.RootfsPath)

	err = archive.CopyWithTar(tmpmnt+`/.`, b.RootfsPath)
	if err != nil {
		return fmt.Errorf("copy Failed: %v", err)
	}

	return nil
}
