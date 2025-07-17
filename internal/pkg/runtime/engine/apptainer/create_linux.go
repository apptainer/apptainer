// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer/rpc/client"
	"github.com/apptainer/apptainer/internal/pkg/util/crypt"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/ccoveille/go-safecast"
	"github.com/pkg/errors"
)

// CreateContainer is called from master process to prepare container
// environment, e.g. perform mount operations, setup network, etc.
//
// Additional privileges required for setup may be gained when running
// in suid flow. However, when a user namespace is requested and it is not
// a hybrid workflow (e.g. fakeroot), then there is no privileged saved uid
// and thus no additional privileges can be gained.
//
// Specifically in apptainer engine, additional privileges are gained during
// network setup (see container.prepareNetworkSetup) in fakeroot flow. The rest
// of the setup (e.g. mount operations) where privileges may be required is performed
// by calling RPC server methods (see internal/app/starter/rpc_linux.go for details).
func (e *EngineOperations) CreateContainer(ctx context.Context, pid int, rpcConn net.Conn) error {
	if e.CommonConfig.EngineName != apptainerConfig.Name {
		return fmt.Errorf("engineName configuration doesn't match runtime name")
	}

	// force the user information for the current (master) process to avoid
	// user database lookup with potential error, see:
	// https://github.com/apptainer/apptainer/issues/2640
	uid32, err := safecast.ToUint32(e.EngineConfig.JSON.UserInfo.UID)
	if err != nil {
		return fmt.Errorf("failed to safely convert UID to uint32: %s", err)
	}
	gid32, err := safecast.ToUint32(e.EngineConfig.JSON.UserInfo.GID)
	if err != nil {
		return fmt.Errorf("failed to safely convert GID to uint32: %s", err)
	}
	user.SetCurrentOriginal(&user.User{
		Name:  e.EngineConfig.JSON.UserInfo.Username,
		UID:   uid32,
		GID:   gid32,
		Gecos: e.EngineConfig.JSON.UserInfo.Gecos,
		Dir:   e.EngineConfig.JSON.UserInfo.Home,
		Shell: e.EngineConfig.JSON.UserInfo.Shell,
	})

	if e.EngineConfig.GetInstanceJoin() {
		return nil
	}

	rpcOps := &client.RPC{
		Client: rpc.NewClient(rpcConn),
		Name:   e.CommonConfig.EngineName,
	}
	if rpcOps.Client == nil {
		return fmt.Errorf("failed to initialize RPC client")
	}

	if err := create(ctx, e, rpcOps, pid); err != nil {
		if strings.Contains(err.Error(), crypt.ErrInvalidPassphrase.Error()) {
			sylog.Debugf("%s", err)
			return errors.New("failed to decrypt, ensure you have supplied appropriate key material")
		}

		if strings.Contains(err.Error(), "mount hook function failure") && strings.Contains(err.Error(), "permission denied") {
			sylog.Infof("Try appending ':ro' to your overlay image or using '--fakeroot'")
		}

		return err
	}

	return nil
}
