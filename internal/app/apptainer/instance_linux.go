// Copyright (c) 2021 Apptainer a Series of LF Projects LLC
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
)

type instanceInfo struct {
	Instance   string `json:"instance"`
	Pid        int    `json:"pid"`
	Image      string `json:"img"`
	IP         string `json:"ip"`
	LogErrPath string `json:"logErrPath"`
	LogOutPath string `json:"logOutPath"`
}

// PrintInstanceList fetches instance list, applying name and
// user filters, and prints it in a regular or a JSON format (if
// formatJSON is true) to the passed writer. Additionally, fetches
// log paths (if showLogs is true).
func PrintInstanceList(w io.Writer, name, user string, formatJSON bool, showLogs bool) error {
	if formatJSON && showLogs {
		sylog.Fatalf("more than one flags have been set")
	}

	tabWriter := tabwriter.NewWriter(w, 0, 8, 4, ' ', 0)
	defer tabWriter.Flush()

	ii, err := instance.List(user, name, instance.AppSubDir)
	if err != nil {
		return fmt.Errorf("could not retrieve instance list: %v", err)
	}

	if showLogs {
		_, err := fmt.Fprintln(tabWriter, "INSTANCE NAME\tPID\tLOGS")
		if err != nil {
			return fmt.Errorf("could not write list header: %v", err)
		}

		for _, i := range ii {
			_, err = fmt.Fprintf(tabWriter, "%s\t%d\t%s\n\t\t%s\n", i.Name, i.Pid, i.LogErrPath, i.LogOutPath)
			if err != nil {
				return fmt.Errorf("could not write instance info: %v", err)
			}
		}
		return nil
	}

	if !formatJSON {
		_, err := fmt.Fprintln(tabWriter, "INSTANCE NAME\tPID\tIP\tIMAGE")
		if err != nil {
			return fmt.Errorf("could not write list header: %v", err)
		}

		for _, i := range ii {
			_, err = fmt.Fprintf(tabWriter, "%s\t%d\t%s\t%s\n", i.Name, i.Pid, i.IP, i.Image)
			if err != nil {
				return fmt.Errorf("could not write instance info: %v", err)
			}
		}
		return nil
	}

	instances := make([]instanceInfo, len(ii))
	for i := range instances {
		instances[i].Image = ii[i].Image
		instances[i].Pid = ii[i].Pid
		instances[i].Instance = ii[i].Name
		instances[i].IP = ii[i].IP
		instances[i].LogErrPath = ii[i].LogErrPath
		instances[i].LogOutPath = ii[i].LogOutPath
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	err = enc.Encode(
		map[string][]instanceInfo{
			"instances": instances,
		})
	if err != nil {
		return fmt.Errorf("could not encode instance list: %v", err)
	}
	return nil
}

// WriteInstancePidFile fetches instance's PID and writes it to the pidFile,
// truncating it if it already exists. Note that the name should not be a glob,
// i.e. name should identify a single instance only, otherwise an error is returned.
func WriteInstancePidFile(name, pidFile string) error {
	inst, err := instance.List("", name, instance.AppSubDir)
	if err != nil {
		return fmt.Errorf("could not retrieve instance list: %v", err)
	}
	if len(inst) != 1 {
		return fmt.Errorf("unexpected instance count: %d", len(inst))
	}

	f, err := os.OpenFile(pidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return fmt.Errorf("could not create pid file: %v", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%d\n", inst[0].Pid)
	if err != nil {
		return fmt.Errorf("could not write pid file: %v", err)
	}
	return nil
}

// StopInstance fetches instance list, applying name and
// user filters, and stops them by sending a signal sig. If an instance
// is still running after a grace period defined by timeout is expired,
// it will be forcibly killed.
func StopInstance(name, user string, sig syscall.Signal, timeout time.Duration) error {
	ii, err := instance.List(user, name, instance.AppSubDir)
	if err != nil {
		return fmt.Errorf("could not retrieve instance list: %v", err)
	}
	if len(ii) == 0 {
		return fmt.Errorf("no instance found")
	}

	stoppedPID := make(chan int, 1)
	stopped := make([]int, 0)

	for _, i := range ii {
		go killInstance(i, sig, stoppedPID)
	}

	for {
		select {
		case pid := <-stoppedPID:
			stopped = append(stopped, pid)
			if len(stopped) == len(ii) {
				return nil
			}
		case <-time.After(timeout):
		killNext:
			for _, i := range ii {
				for _, pid := range stopped {
					if i.Pid == pid {
						continue killNext
					}
				}

				sylog.Infof("Killing %s instance of %s (PID=%d) (Timeout)\n", i.Name, i.Image, i.Pid)
				syscall.Kill(i.Pid, syscall.SIGKILL)
			}
			return nil
		}
	}
}

func killInstance(i *instance.File, sig syscall.Signal, stoppedPID chan<- int) {
	sylog.Infof("Stopping %s instance of %s (PID=%d)\n", i.Name, i.Image, i.Pid)
	syscall.Kill(i.Pid, sig)

	for {
		if err := syscall.Kill(i.PPid, 0); err == syscall.ESRCH {
			stoppedPID <- i.Pid
			break
		}
		if childs, err := proc.CountChilds(i.Pid); childs == 0 {
			if err == nil {
				syscall.Kill(i.Pid, syscall.SIGKILL)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
