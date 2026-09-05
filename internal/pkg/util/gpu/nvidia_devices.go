// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package gpu

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// The trees the driver's GPUs, their PCI devices and the device nodes are
// read from; the tests point them at their own.
var (
	nvidiaProcDir = "/proc/driver/nvidia/gpus"
	pciDevicesDir = "/sys/bus/pci/devices"
	devDir        = "/dev"
)

// nvidiaControlMinor is the minor number of the driver's control device
// node, /dev/nvidiactl.
const nvidiaControlMinor = 255

// nvidiaGPU is a GPU as the loaded driver reports it.
type nvidiaGPU struct {
	busID string
	uuid  string
	minor int
}

// nvidiaGPUs lists the GPUs the loaded driver reports and has not excluded,
// in PCI bus order, which is the order nvidia-container-cli indexes them in.
func nvidiaGPUs() ([]nvidiaGPU, error) {
	entries, err := os.ReadDir(nvidiaProcDir)
	if err != nil {
		return nil, err
	}
	gpus := make([]nvidiaGPU, 0, len(entries))
	for _, entry := range entries {
		info, err := os.ReadFile(filepath.Join(nvidiaProcDir, entry.Name(), "information"))
		if err != nil {
			sylog.Debugf("Skipping GPU %s: %v", entry.Name(), err)
			continue
		}
		gpu := nvidiaGPU{busID: entry.Name(), minor: -1}
		excluded := false
		for _, line := range strings.Split(string(info), "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.TrimSpace(key) {
			case "GPU UUID":
				gpu.uuid = value
			case "Device Minor":
				if minor, err := strconv.Atoi(value); err == nil {
					gpu.minor = minor
				}
			case "GPU Excluded":
				excluded = strings.EqualFold(value, "yes")
			}
		}
		if !excluded {
			gpus = append(gpus, gpu)
		}
	}
	slices.SortFunc(gpus, func(a, b nvidiaGPU) int { return strings.Compare(a.busID, b.busID) })
	return gpus, nil
}

// pciAddress matches the PCI address form nvidia-container-cli accepts:
// domain, bus and device in hexadecimal, leading zeros optional, and the
// function ignored as the CLI ignores it.
var pciAddress = regexp.MustCompile(`^([0-9A-Fa-f]{1,8}):([0-9A-Fa-f]{1,2}):([0-9A-Fa-f]{1,2})(?:\.[0-9A-Fa-f])?$`)

// normalizePCIAddress writes the domain, bus and device of a PCI address
// without leading zeros.
func normalizePCIAddress(address string) (string, bool) {
	m := pciAddress.FindStringSubmatch(address)
	if m == nil {
		return "", false
	}
	parts := make([]string, 0, len(m)-1)
	for _, field := range m[1:] {
		n, _ := strconv.ParseUint(field, 16, 32)
		parts = append(parts, strconv.FormatUint(n, 16))
	}
	return strings.Join(parts, ":"), true
}

// namesNvidiaGPU reports whether one NVIDIA_VISIBLE_DEVICES entry names the
// GPU at the given index: that index, its "GPU-" UUID or its PCI address; a
// MIG index such as "0:1" names its GPU.
func namesNvidiaGPU(name string, index int, gpu nvidiaGPU) bool {
	if address, ok := normalizePCIAddress(name); ok {
		busID, _ := normalizePCIAddress(gpu.busID)
		return address == busID
	}
	if strings.HasPrefix(strings.ToUpper(name), "GPU-") {
		return strings.EqualFold(name, gpu.uuid)
	}
	position, rest, isMIG := strings.Cut(name, ":")
	if n, err := strconv.ParseUint(position, 10, 32); err != nil || int(n) != index {
		return false
	}
	if isMIG {
		_, err := strconv.ParseUint(rest, 10, 32)
		return err == nil
	}
	return true
}

// selectNvidiaGPUs picks the GPUs an NVIDIA_VISIBLE_DEVICES value names,
// read as nvidia-container-cli reads it: "all", or a comma-separated list of
// entries each naming one GPU. "none", "void" and an empty value name no GPU.
func selectNvidiaGPUs(visible string, gpus []nvidiaGPU) []nvidiaGPU {
	var selected []nvidiaGPU
	for _, name := range strings.Split(visible, ",") {
		name = strings.TrimSpace(name)
		if name == "" || strings.EqualFold(name, "none") || strings.EqualFold(name, "void") {
			continue
		}
		if strings.EqualFold(name, "all") {
			return gpus
		}
		for i, gpu := range gpus {
			if namesNvidiaGPU(name, i, gpu) && !slices.Contains(selected, gpu) {
				selected = append(selected, gpu)
			}
		}
	}
	return selected
}

// NvidiaDrmDevices returns the DRM device nodes of the GPUs an
// NVIDIA_VISIBLE_DEVICES value names, the card and render nodes sysfs lists
// under each GPU's PCI device. They are what GBM, the EGL device platform and
// an X server open on the GPU; nvidia-container-cli stages the driver's own
// nodes only.
func NvidiaDrmDevices(visible string) []string {
	gpus, err := nvidiaGPUs()
	if err != nil {
		sylog.Debugf("No GPU reported by the NVIDIA driver: %v", err)
		return nil
	}
	var nodes []string
	for _, gpu := range selectNvidiaGPUs(visible, gpus) {
		entries, err := os.ReadDir(filepath.Join(pciDevicesDir, gpu.busID, "drm"))
		if err != nil {
			sylog.Debugf("No DRM device for GPU %s: %v", gpu.busID, err)
			continue
		}
		for _, entry := range entries {
			node := filepath.Join(devDir, "dri", entry.Name())
			if _, err := os.Stat(node); err == nil {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes
}

// nvidiaDeviceNode is a device node of the driver with the nvidia-modprobe
// arguments that create it.
type nvidiaDeviceNode struct {
	path string
	args []string
}

// nvidiaDeviceNodes lists the driver's device nodes for the GPUs it reports:
// the control node, one per GPU, the unified memory pair from its base minor
// and the modeset node.
func nvidiaDeviceNodes(gpus []nvidiaGPU) []nvidiaDeviceNode {
	nodes := []nvidiaDeviceNode{
		{filepath.Join(devDir, "nvidiactl"), []string{"-c=" + strconv.Itoa(nvidiaControlMinor)}},
	}
	for _, gpu := range gpus {
		if gpu.minor < 0 {
			continue
		}
		minor := strconv.Itoa(gpu.minor)
		nodes = append(nodes, nvidiaDeviceNode{filepath.Join(devDir, "nvidia"+minor), []string{"-c=" + minor}})
	}
	return append(nodes,
		nvidiaDeviceNode{filepath.Join(devDir, "nvidia-uvm"), []string{"-u", "-c=0"}},
		nvidiaDeviceNode{filepath.Join(devDir, "nvidia-uvm-tools"), []string{"-u", "-c=0"}},
		nvidiaDeviceNode{filepath.Join(devDir, "nvidia-modeset"), []string{"-m"}},
	)
}

// NvidiaCreateDevices creates the driver's device nodes the host lacks
// through nvidia-modprobe, the setuid helper the driver installs so that a
// process without privilege can create them, which the driver's libraries
// call on first use the same way. Nothing else on a host makes the modeset
// node before something asks for it, and a container without it cannot
// present through Vulkan. A helper that is absent or refuses leaves the
// nodes as they are.
func NvidiaCreateDevices() {
	gpus, err := nvidiaGPUs()
	if err != nil {
		sylog.Debugf("No GPU reported by the NVIDIA driver: %v", err)
		return
	}
	helper := ""
	for _, node := range nvidiaDeviceNodes(gpus) {
		if _, err := os.Stat(node.path); err == nil {
			continue
		}
		if helper == "" {
			if helper, err = bin.FindBin("nvidia-modprobe"); err != nil {
				sylog.Debugf("Not creating %s: %v", node.path, err)
				return
			}
		}
		sylog.Debugf("Creating %s with nvidia-modprobe %s", node.path, strings.Join(node.args, " "))
		if out, err := exec.Command(helper, node.args...).CombinedOutput(); err != nil {
			sylog.Verbosef("Could not create %s: %v %s", node.path, err, bytes.TrimSpace(out))
		}
	}
}
