// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	osignal "os/signal"
	"sync"
	"syscall"

	"github.com/creack/pty"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/oci"
	"github.com/apptainer/apptainer/pkg/ociruntime"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/unix"
	"github.com/ccoveille/go-safecast"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/term"
)

func resize(controlSocket string, oversized bool) {
	ctrl := &ociruntime.Control{}
	ctrl.ConsoleSize = &specs.Box{}

	c, err := unix.Dial(controlSocket)
	if err != nil {
		sylog.Errorf("failed to connect to control socket")
		return
	}
	defer c.Close()

	rows, cols, err := pty.Getsize(os.Stdin)
	if err != nil {
		sylog.Errorf("terminal resize error: %s", err)
		return
	}

	urows, err := safecast.Convert[uint](rows)
	if err != nil {
		sylog.Errorf("failed to convert rows to uint: %s", err)
		return
	}
	ucols, err := safecast.Convert[uint](cols)
	if err != nil {
		sylog.Errorf("failed to convert columns to uint: %s", err)
		return
	}
	ctrl.ConsoleSize.Height = urows
	ctrl.ConsoleSize.Width = ucols

	if oversized {
		ctrl.ConsoleSize.Height++
		ctrl.ConsoleSize.Width++
	}

	enc := json.NewEncoder(c)
	if enc == nil {
		sylog.Errorf("cannot instantiate JSON encoder")
		return
	}

	if err := enc.Encode(ctrl); err != nil {
		sylog.Errorf("%s", err)
		return
	}
}

func attach(engineConfig *oci.EngineConfig, run bool) error {
	var ostate *term.State
	var conn net.Conn
	var wg sync.WaitGroup

	state := &engineConfig.State

	if state.AttachSocket == "" {
		return fmt.Errorf("attach socket not available, container state: %s", state.Status)
	}
	if state.ControlSocket == "" {
		return fmt.Errorf("control socket not available, container state: %s", state.Status)
	}

	hasTerminal := engineConfig.OciConfig.Process.Terminal
	if hasTerminal && !term.IsTerminal(0) {
		return fmt.Errorf("attach requires a terminal when terminal config is set to true")
	}

	var err error
	conn, err = unix.Dial(state.AttachSocket)
	if err != nil {
		return err
	}
	defer conn.Close()

	if hasTerminal {
		ostate, _ = term.MakeRaw(0)
		resize(state.ControlSocket, true)
		resize(state.ControlSocket, false)
	}

	wg.Add(1)

	go func() {
		// catch SIGWINCH signal for terminal resize
		signals := make(chan os.Signal, 1)
		pid := state.Pid
		osignal.Notify(signals)

		for {
			s := <-signals
			switch s {
			case syscall.SIGWINCH:
				if hasTerminal {
					resize(state.ControlSocket, false)
				}
			default:
				syscall.Kill(pid, s.(syscall.Signal))
			}
		}
	}()

	if hasTerminal || !run {
		// Pipe session to bash and visa-versa
		go func() {
			io.Copy(os.Stdout, conn)
			wg.Done()
		}()
		go func() {
			io.Copy(conn, os.Stdin)
		}()
		wg.Wait()

		if hasTerminal {
			fmt.Printf("\r")
			return term.Restore(0, ostate)
		}
		return nil
	}

	io.Copy(io.Discard, conn)
	return nil
}

// OciAttach attaches console to a running container
func OciAttach(ctx context.Context, containerID string) error {
	engineConfig, err := getEngineConfig(containerID)
	if err != nil {
		return err
	}
	if engineConfig.GetState().Status != ociruntime.Running {
		return fmt.Errorf("could not attach to %s: not in running state", containerID)
	}

	defer exitContainer(ctx, containerID, false)

	return attach(engineConfig, false)
}
