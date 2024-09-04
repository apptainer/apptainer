// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package dmtcp

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
)

type Entry struct {
	path string
}

func (e *Entry) CoordinatorPort() (string, error) {
	f, err := os.Open(filepath.Join(e.Path(), portFile))
	if err != nil {
		return "", err
	}
	defer f.Close()

	// scan first line of file for port
	s := bufio.NewScanner(f)
	s.Scan()
	if s.Text() == "" {
		return "", fmt.Errorf("unable to scan port from checkpoint data")
	}

	return s.Text(), nil
}

func (e *Entry) BindPath() apptainerConfig.BindPath {
	return apptainerConfig.BindPath{
		Source:      e.path,
		Destination: containerStatepath,
		Options: map[string]*apptainerConfig.BindOption{
			"rw": {},
		},
	}
}

func (e *Entry) Path() string {
	return e.path
}

func (e *Entry) Name() string {
	return filepath.Base(e.path)
}

type Manager interface {
	Create(string) (*Entry, error) // create checkpoint directory for dmtcp state
	Get(string) (*Entry, error)    // ensure directory with dmtcp state exists
	List() ([]*Entry, error)       // list checkpoint directories for dmtcp state
	Delete(string) error           // delete checkpoint directory for dmtcp state
}

type checkpointManager struct{}

func NewManager() Manager {
	return &checkpointManager{}
}

func (checkpointManager) Create(name string) (*Entry, error) {
	err := os.MkdirAll(filepath.Join(dmtcpDir(), name), 0o700)
	if err != nil {
		return nil, err
	}

	return &Entry{filepath.Join(dmtcpDir(), name)}, nil
}

func (checkpointManager) Get(name string) (*Entry, error) {
	if name == "" {
		return nil, fmt.Errorf("checkpoint name must not be empty")
	}

	_, err := os.Stat(filepath.Join(dmtcpDir(), name))
	if err != nil {
		return nil, err
	}

	return &Entry{filepath.Join(dmtcpDir(), name)}, nil
}

func (checkpointManager) List() ([]*Entry, error) {
	fis, err := os.ReadDir(dmtcpDir())
	if err != nil {
		return nil, err
	}

	entries := make([]*Entry, 0, len(fis))
	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		}

		entries = append(entries, &Entry{filepath.Join(dmtcpDir(), fi.Name())})
	}

	return entries, nil
}

func (checkpointManager) Delete(name string) error {
	_, err := os.Stat(filepath.Join(dmtcpDir(), name))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("checkpoint %q not found", name)
		}
	}

	return os.RemoveAll(filepath.Join(dmtcpDir(), name))
}
