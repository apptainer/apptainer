// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package instance

import (
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/cenkalti/backoff/v4"
)

const checkpointStateServerPort = 11000

func pollServer(t *testing.T, address string) {
	op := func() error {
		resp, err := http.Get(address)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	b := backoff.WithMaxRetries(
		backoff.NewConstantBackOff(1*time.Second), // Ping every second.
		30, // Ping for a total of 30 seconds.
	)

	err := backoff.Retry(op, b)
	if err != nil {
		t.Fatalf("Unable to reach server after 30s: %v", err)
	}
}

func getServerState(t *testing.T, address, expected string) {
	resp, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if string(body) != expected {
		t.Fatalf("Expected %q, got %q", expected, string(body))
	}
}

func setServerState(t *testing.T, address, val string) {
	resp, err := http.Post(address, "text/plain", strings.NewReader(val))
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if string(body) != val {
		t.Fatalf("Expected %q, got %q", val, string(body))
	}
}

// testCheckpointInstance runs through a basic checkpointing scenario with a python server
// that stores a variable in memory.
// NOTE(ian): The excessive sleep times are necessary when I run these tests locally since
// I get a "connect: connection refused" error when it is significantly shortened. It is
// unclear to my why this is the case as manual testing does not appear to require such delays.
func (c *ctx) testCheckpointInstance(t *testing.T) {
	require.DMTCP(t)

	imageDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "checkpoint-", "")
	defer e2e.Privileged(cleanup)(t)

	imagePath := filepath.Join(imageDir, "state-server.sif")
	checkpointName := randomName(t)
	instanceName := randomName(t)
	instanceAddress := "http://" + net.JoinHostPort("localhost", strconv.Itoa(checkpointStateServerPort))

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", imagePath, "testdata/state-server.def"),
		e2e.ExpectExit(0),
	)

	// Create checkpoint
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("checkpoint"),
		e2e.WithArgs("create", checkpointName),
		e2e.ExpectExit(0),
	)

	// Start instance using the checkpoint with "--dmtcp-launch"
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("instance"),
		e2e.WithArgs("start", "--dmtcp-launch", checkpointName, imagePath, instanceName, strconv.Itoa(checkpointStateServerPort)),
		e2e.ExpectExit(0),
	)

	// Wait for server to come up
	pollServer(t, instanceAddress)

	// Check that server state is initialized to what we expect
	getServerState(t, instanceAddress, "0")

	// Set server state to something new before checkpointing
	setServerState(t, instanceAddress, "1")

	// Checkpoint instance
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("checkpoint"),
		e2e.WithArgs("instance", instanceName),
		e2e.ExpectExit(0),
	)

	// Give the checkpoint command some time to save state
	time.Sleep(5 * time.Second)

	// Stop instance
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("instance"),
		e2e.WithArgs("stop", instanceName),
		e2e.ExpectExit(0),
	)

	// Wait for socket to completely drain before restarting.
	time.Sleep(1 * time.Minute)

	// Start instance using the checkpoint with "--dmtcp-restart"
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("instance"),
		e2e.WithArgs("start", "--dmtcp-restart", checkpointName, imagePath, instanceName),
		e2e.ExpectExit(0),
	)

	// Wait for server to come up
	pollServer(t, instanceAddress)

	// Ensure server state after restart is what we set it to before checkpoint
	getServerState(t, instanceAddress, "1")

	// Stop instance
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("instance"),
		e2e.WithArgs("stop", instanceName),
		e2e.ExpectExit(0),
	)

	// Delete checkpoint
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("checkpoint"),
		e2e.WithArgs("delete", checkpointName),
		e2e.ExpectExit(0),
	)
}
