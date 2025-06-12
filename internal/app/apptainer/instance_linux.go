// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Includes code from https://github.com/docker/cli
// Released under the Apache License Version 2.0

package apptainer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/cgroups"
	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	"github.com/buger/goterm"
	"github.com/ccoveille/go-safecast"
	units "github.com/docker/go-units"
	libcgroups "github.com/opencontainers/cgroups"
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
func PrintInstanceList(w io.Writer, name, user string, formatJSON bool, showLogs bool, all bool) error {
	if formatJSON && showLogs {
		sylog.Fatalf("more than one flags have been set")
	}

	tabWriter := tabwriter.NewWriter(w, 0, 8, 4, ' ', 0)
	defer tabWriter.Flush()

	ii, err := instance.List(user, name, instance.AppSubDir, all)
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
	inst, err := instance.List("", name, instance.AppSubDir, true)
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

// instanceListOrError is a private function to retrieve named instances or fail if there are no instances
// We wrap the error from instance.List to provide a more specific error message
func instanceListOrError(instanceUser, name string) ([]*instance.File, error) {
	ii, err := instance.List(instanceUser, name, instance.AppSubDir, true)
	if err != nil {
		return ii, fmt.Errorf("could not retrieve instance list: %w", err)
	}
	if len(ii) == 0 {
		return ii, fmt.Errorf("no instance found")
	}
	return ii, err
}

// calculate BlockIO counts up read/write totals
func calculateBlockIO(stats *libcgroups.BlkioStats) (float64, float64) {
	var read, write float64
	for _, entry := range stats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			read += float64(entry.Value)
		case "write":
			write += float64(entry.Value)
		}
	}
	return read, write
}

// calculateMemoryUsage returns the current usage, limit, and percentage
func calculateMemoryUsage(stats *libcgroups.MemoryStats) (float64, float64, float64) {
	// Note that there is also MaxUsage
	memUsage := stats.Usage.Usage
	memLimit := stats.Usage.Limit
	memPercent := 0.0

	// If there is no limit, show system RAM instead of max uint64...
	if memLimit == math.MaxUint64 {
		in := &syscall.Sysinfo_t{}
		err := syscall.Sysinfo(in)
		if err == nil {
			memLimit = uint64(in.Totalram) * uint64(in.Unit)
		}
	}
	if memLimit != 0 {
		memPercent = float64(memUsage) / float64(memLimit) * 100.0
	}
	return float64(memUsage), float64(memLimit), memPercent
}

func calculateCPUUsage(prevTime, prevCPU uint64, cpuStats *libcgroups.CpuStats) (cpuPercent float64, curTime, curCPU uint64, err error) {
	// Update 1s interval CPU ns usage
	curTime, err = safecast.ToUint64(time.Now().UnixNano())
	if err != nil {
		return 0, 0, 0, err
	}
	curCPU = cpuStats.CpuUsage.TotalUsage
	deltaCPU := float64(curCPU - prevCPU)
	deltaTime := float64(curTime - prevTime)
	cpuPercent = (deltaCPU / deltaTime) * 100
	return cpuPercent, curTime, curCPU, nil
}

// InstanceStats uses underlying cgroups to get statistics for a named instance
func InstanceStats(ctx context.Context, name, instanceUser string, formatJSON bool, noStream bool) error {
	ii, err := instanceListOrError(instanceUser, name)
	if err != nil {
		return err
	}
	// Instance stats required 1 instance
	if len(ii) != 1 {
		return fmt.Errorf("query returned more than one instance (%d)", len(ii))
	}

	// Grab our instance to interact with!
	i := ii[0]
	if !formatJSON {
		sylog.Infof("Stats for %s instance of %s (PID=%d)\n", i.Name, i.Image, i.Pid)
	}

	// If asking for json and not nostream, not possible
	if formatJSON && !noStream {
		sylog.Warningf("JSON output is only available for a single timepoint (--no-stream)")
		noStream = true
	}

	// Cut out early if we do not have cgroups
	if !i.Cgroup {
		url := "the Apptainer instance user guide for instructions"
		return fmt.Errorf("stats are only available if cgroups are enabled, see %s", url)
	}

	// Get a cgroupfs managed cgroup from the pid
	manager, err := cgroups.GetManagerForPid(i.Pid)
	if err != nil {
		return fmt.Errorf("while getting cgroup manager for pid: %v", err)
	}

	// Otherwise print shortened table
	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
	defer tabWriter.Flush()

	// Retrieve initial state, for first CPU measurement
	stats, err := manager.GetStats()
	if err != nil {
		return fmt.Errorf("while getting stats for pid: %v", err)
	}
	prevCPU := stats.CpuStats.CpuUsage.TotalUsage
	prevTime, err := safecast.ToUint64(time.Now().UnixNano())
	if err != nil {
		return err
	}
	cpuPercent := 0.0

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-time.After(1 * time.Second):

			// Stream clears the terminal and reprint header and stats each time
			if !noStream {
				goterm.Clear()
				goterm.MoveCursor(1, 1)
				goterm.Flush()
			}

			// Retrieve new stats
			stats, err := manager.GetStats()
			if err != nil {
				return fmt.Errorf("while getting stats for pid: %v", err)
			}

			// Do we want json?
			if formatJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "\t")
				err = enc.Encode(stats)
				return err
			}

			// Stats can be added from this set
			// https://github.com/opencontainers/cgroups/blob/main/stats.go
			_, err = fmt.Fprintln(tabWriter, "INSTANCE NAME\tCPU USAGE\tMEM USAGE / LIMIT\tMEM %\tBLOCK I/O\tPIDS")
			if err != nil {
				return fmt.Errorf("could not write stats header: %v", err)
			}

			cpuPercent, prevTime, prevCPU, err = calculateCPUUsage(prevTime, prevCPU, &stats.CpuStats)
			if err != nil {
				return err
			}
			memUsage, memLimit, memPercent := calculateMemoryUsage(&stats.MemoryStats)
			blockRead, blockWrite := calculateBlockIO(&stats.BlkioStats)

			// Generate a shortened stats list
			_, err = fmt.Fprintf(tabWriter, "%s\t%.2f%%\t%s / %s\t%.2f%s\t%s / %s\t%d\n", i.Name,
				cpuPercent, units.BytesSize(memUsage), units.BytesSize(memLimit),
				memPercent, "%", units.BytesSize(blockRead), units.BytesSize(blockWrite),
				stats.PidsStats.Current)
			tabWriter.Flush()
			if err != nil {
				return fmt.Errorf("could not write instance stats: %v", err)
			}

			// We don't want a stream, return after just one record
			if noStream {
				return nil
			}
		}
	}
}

// StopInstance fetches instance list, applying name and
// user filters, and stops them by sending a signal sig. If an instance
// is still running after a grace period defined by timeout is expired,
// it will be forcibly killed.
func StopInstance(name, user string, sig syscall.Signal, timeout time.Duration) error {
	ii, err := instanceListOrError(user, name)
	if err != nil {
		return err
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
