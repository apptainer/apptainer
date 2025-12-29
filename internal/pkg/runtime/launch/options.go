// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package launcher is responsible for starting a container, with configuration
// passed to it from the CLI layer.
//
// The package currently implements a single Launcher, with an Exec method that
// constructs a runtime configuration and calls the Apptainer runtime starter
// binary to start the container.
//
// TODO - the launcher package will be extended to support launching containers
// via the OCI runc/crun runtime, in addition to the current Apptainer runtime
// starter.
package launch

import (
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
	"github.com/apptainer/apptainer/pkg/util/cryptkey"
)

// launchOptions accumulates configuration from passed functional options. Note
// that the launchOptions is modified heavily by logic during the Exec function
// call.
type launchOptions struct {
	// Writable marks the container image itself as writable.
	Writable bool
	// WriteableTmpfs applies an ephemeral writable overlay to the container.
	WritableTmpfs bool
	// OverlayPaths holds paths to image or directory overlays to be applied.
	OverlayPaths []string
	// Scratchdir lists paths into the container to be mounted from a temporary location on the host.
	ScratchDirs []string
	// WorkDir is the parent path for scratch directories, and contained home/tmp on the host.
	WorkDir string

	// HomeDir is the home directory to mount into the container, or a src:dst pair.
	HomeDir string
	// CustomHome is a marker that HomeDir is user-supplied, and should not be
	// modified by the logic used for fakeroot execution.
	CustomHome bool
	// NoHome disables automatic mounting of the home directory into the container.
	NoHome bool

	// BindPaths lists paths to bind from host to container, which may be <src>:<dest> pairs.
	BindPaths []string
	// FuseMount lists paths to be mounted into the container using a FUSE binary, and their options.
	FuseMount []string
	// Mounts lists paths to bind from host to container, from the docker compatible `--mount` flag (CSV format).
	Mounts []string
	// NoMount is a list of automatic / configured mounts to disable.
	NoMount []string

	// Nvidia enables NVIDIA GPU support.
	Nvidia bool
	// NcCCLI sets NVIDIA GPU support to use the nvidia-container-cli.
	NvCCLI bool
	// NoNvidia disables NVIDIA GPU support when set default in apptainer.conf.
	NoNvidia bool
	// Rocm enables Rocm GPU support.
	Rocm bool
	// NoRocm disable Rocm GPU support when set default in apptainer.conf.
	NoRocm bool

	// ContainLibs lists paths of libraries to bind mount into the container .singularity.d/libs dir.
	ContainLibs []string

	// Env is a map of name=value env vars to set in the container.
	Env map[string]string
	// EnvFiles contains filenames to read container env vars from.
	EnvFiles []string
	// CleanEnv starts the container with a clean environment, excluding host env vars.
	CleanEnv bool
	// NoEval instructs Apptainer not to shell evaluate args and env vars.
	NoEval bool

	// Namespaces is the list of optional Namespaces requested for the container.
	Namespaces Namespaces

	// NetnsPath is the path to a network namespace to join, rather than
	// creating one / applying a CNI config.
	NetnsPath string

	// Network is the name of an optional CNI networking configuration to apply.
	Network string
	// NetworkArgs are argument to pass to the CNI plugin that will configure networking when Network is set.
	NetworkArgs []string
	// Hostname is the hostname to set in the container (infers/requires UTS namespace).
	Hostname string
	// DNS is the comma separated list of DNS servers to be set in the container's resolv.conf.
	DNS string

	// AddCaps is the list of capabilities to Add to the container process.
	AddCaps string
	// DropCaps is the list of capabilities to drop from the container process.
	DropCaps string
	// AllowSUID permits setuid executables inside a container started by the root user.
	AllowSUID bool
	// KeepPrivs keeps all privileges inside a container started by the root user.
	KeepPrivs bool
	// NoPrivs drops all privileges inside a container.
	NoPrivs bool
	// SecurityOpts is the list of security options (selinux, apparmor, seccomp) to apply.
	SecurityOpts []string
	// NoUmask disables propagation of the host umask into the container, using a default 0022.
	NoUmask bool

	// CGroupsJSON is a JSON format cgroups resource limit specification to apply.
	CGroupsJSON string

	// ConfigFile is an alternate apptainer.conf that will be used by unprivileged installations only.
	ConfigFile string

	// ShellPath is a custom shell executable to be launched in the container.
	ShellPath string
	// CwdPath is the initial working directory in the container.
	CwdPath string

	// Fakeroot enables the fake root mode, using user namespaces and subuid / subgid mapping.
	Fakeroot bool
	// Boot enables execution of /sbin/init on startup of an instance container.
	Boot bool
	// NoInit disables shim process when PID namespace is used.
	NoInit bool
	// Contain starts the container with minimal /dev and empty home/tmp mounts.
	Contain bool
	// ContainAll infers Contain, and adds PID, IPC namespaces, and CleanEnv.
	ContainAll bool

	// AppName sets a SCIF application name to run.
	AppName string

	// KeyInfo holds encryption key information for accessing encrypted containers.
	KeyInfo *cryptkey.KeyInfo

	// SIFFUSE enables mounting SIF container images using FUSE.
	SIFFUSE bool
	// CacheDisabled indicates caching of images was disabled in the CLI, as in
	// userns flows we will need to delete the redundant temporary pulled image after
	// conversion to sandbox.
	CacheDisabled bool

	DMTCPLaunch       string
	DMTCPRestart      string
	Unsquash          bool
	IgnoreSubuid      bool
	IgnoreFakerootCmd bool
	IgnoreUserns      bool
	UseBuildConfig    bool
	TmpDir            string
	Underlay          bool   // whether prefer underlay over overlay
	ShareNSMode       bool   // whether running in sharens mode
	ShareNSFd         int    // fd opened in sharens mode
	RunscriptTimeout  string // runscript timeout

	// Devices lists fully-qualified CDI device names to make available in the container.
	Devices []string
	// CdiDirs lists directories in which CDI should look for device definition JSON files.
	CdiDirs []string

	// IntelHpu enables Intel(R) Gaudi accelerator support.
	IntelHpu bool
}

type Launcher struct {
	uid          uint32
	gid          uint32
	cfg          launchOptions
	engineConfig *apptainerConfig.EngineConfig
	generator    *generate.Generator
}

// Namespaces holds flags for the optional (non-mount) namespaces that can be
// requested for a container launch.
type Namespaces struct {
	User bool
	UTS  bool
	PID  bool
	IPC  bool
	Net  bool
	// NoPID will force the PID namespace not to be used, even if set by default / other flags.
	NoPID bool
}

type Option func(co *launchOptions) error

// OptWritable sets the container image to be writable.
func OptWritable(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Writable = b
		return nil
	}
}

// OptWritableTmpFs applies an ephemeral writable overlay to the container.
func OptWritableTmpfs(b bool) Option {
	return func(lo *launchOptions) error {
		lo.WritableTmpfs = b
		return nil
	}
}

// OptOverlayPaths sets overlay images and directories to apply to the container.
func OptOverlayPaths(op []string) Option {
	return func(lo *launchOptions) error {
		lo.OverlayPaths = op
		return nil
	}
}

// OptScratchDirs sets temporary host directories to create and bind into the container.
func OptScratchDirs(sd []string) Option {
	return func(lo *launchOptions) error {
		lo.ScratchDirs = sd
		return nil
	}
}

// OptWorkDir sets the parent path for scratch directories, and contained home/tmp on the host.
func OptWorkDir(wd string) Option {
	return func(lo *launchOptions) error {
		lo.WorkDir = wd
		return nil
	}
}

// OptHome sets the home directory configuration for the container.
//
// homeDir is the path or src:dst to bind mount.
// custom is a marker that this is user supplied, and must not be overridden.
// disable will disable the home mount entirely, ignoring other options.
func OptHome(homeDir string, custom bool, disable bool) Option {
	return func(lo *launchOptions) error {
		lo.HomeDir = homeDir
		lo.CustomHome = custom
		lo.NoHome = disable
		return nil
	}
}

// OptMounts sets user-requested mounts to propagate into the container.
//
// binds lists bind mount specifications in Apptainer's <src>:<dst>[:<opts>] format.
// mounts lists bind mount specifications in Docker CSV processed format.
// fuseMounts list FUSE mounts in <type>:<fuse command> <mountpoint> format.
func OptMounts(binds []string, mounts []string, fuseMounts []string) Option {
	return func(lo *launchOptions) error {
		lo.BindPaths = binds
		lo.Mounts = mounts
		lo.FuseMount = fuseMounts
		return nil
	}
}

// OptNoMount disables the specified bind mounts.
func OptNoMount(nm []string) Option {
	return func(lo *launchOptions) error {
		lo.NoMount = nm
		return nil
	}
}

// OptNvidia enables NVIDIA GPU support.
//
// nvccli sets whether to use the nvidia-container-runtime (true), or legacy bind mounts (false).
func OptNvidia(nv bool, nvccli bool) Option {
	return func(lo *launchOptions) error {
		lo.Nvidia = nv || nvccli
		lo.NvCCLI = nvccli
		return nil
	}
}

// OptNoNvidia disables NVIDIA GPU support, even if enabled via apptainer.conf.
func OptNoNvidia(b bool) Option {
	return func(lo *launchOptions) error {
		lo.NoNvidia = b
		return nil
	}
}

// OptRocm enable Rocm GPU support.
func OptRocm(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Rocm = b
		return nil
	}
}

// OptNoRocm disables Rocm GPU support, even if enabled via apptainer.conf.
func OptNoRocm(b bool) Option {
	return func(lo *launchOptions) error {
		lo.NoRocm = b
		return nil
	}
}

// OptContainLibs mounts specified libraries into the container .singularity.d/libs dir.
func OptContainLibs(cl []string) Option {
	return func(lo *launchOptions) error {
		lo.ContainLibs = cl
		return nil
	}
}

// OptEnv sets container environment
//
// envFiles is a slice of paths to files container environment variables to set
// env is a map of name=value env vars to set.
// clean removes host variables from the container environment.
func OptEnv(env map[string]string, envFiles []string, clean bool) Option {
	return func(lo *launchOptions) error {
		lo.Env = env
		lo.EnvFiles = envFiles
		lo.CleanEnv = clean
		return nil
	}
}

// OptNoEval disables shell evaluation of args and env vars.
func OptNoEval(b bool) Option {
	return func(lo *launchOptions) error {
		lo.NoEval = b
		return nil
	}
}

// OptNamespaces enable the individual kernel-support namespaces for the container.
func OptNamespaces(n Namespaces) Option {
	return func(lo *launchOptions) error {
		lo.Namespaces = n
		return nil
	}
}

// OptJoinNetNamespace sets the network namespace to join, if permitted.
func OptNetnsPath(n string) Option {
	return func(lo *launchOptions) error {
		lo.NetnsPath = n
		return nil
	}
}

// OptNetwork enables CNI networking.
//
// network is the name of the CNI configuration to enable.
// args are arguments to pass to the CNI plugin.
func OptNetwork(network string, args []string) Option {
	return func(lo *launchOptions) error {
		lo.Network = network
		lo.NetworkArgs = args
		return nil
	}
}

// OptHostname sets a hostname for the container (infers/requires UTS namespace).
func OptHostname(h string) Option {
	return func(lo *launchOptions) error {
		lo.Hostname = h
		return nil
	}
}

// OptDNS sets a DNS entry for the container resolv.conf.
func OptDNS(d string) Option {
	return func(lo *launchOptions) error {
		lo.DNS = d
		return nil
	}
}

// OptCaps sets capabilities to add and drop.
func OptCaps(add, drop string) Option {
	return func(lo *launchOptions) error {
		lo.AddCaps = add
		lo.DropCaps = drop
		return nil
	}
}

// OptAllowSUID permits setuid executables inside a container started by the root user.
func OptAllowSUID(b bool) Option {
	return func(lo *launchOptions) error {
		lo.AllowSUID = b
		return nil
	}
}

// OptKeepPrivs keeps all privileges inside a container started by the root user.
func OptKeepPrivs(b bool) Option {
	return func(lo *launchOptions) error {
		lo.KeepPrivs = b
		return nil
	}
}

// OptNoPrivs drops all privileges inside a container.
func OptNoPrivs(b bool) Option {
	return func(lo *launchOptions) error {
		lo.NoPrivs = b
		return nil
	}
}

// OptSecurity supplies a list of security options (selinux, apparmor, seccomp) to apply.
func OptSecurity(s []string) Option {
	return func(lo *launchOptions) error {
		lo.SecurityOpts = s
		return nil
	}
}

// OptNoUmask disables propagation of the host umask into the container, using a default 0022.
func OptNoUmask(b bool) Option {
	return func(lo *launchOptions) error {
		lo.NoUmask = b
		return nil
	}
}

// OptCgroupsJSON sets a Cgroups resource limit configuration to apply to the container.
func OptCgroupsJSON(cj string) Option {
	return func(lo *launchOptions) error {
		lo.CGroupsJSON = cj
		return nil
	}
}

// OptConfigFile specifies an alternate apptainer.conf that will be used by unprivileged installations only.
func OptConfigFile(c string) Option {
	return func(lo *launchOptions) error {
		lo.ConfigFile = c
		return nil
	}
}

// OptShellPath specifies a custom shell executable to be launched in the container.
func OptShellPath(s string) Option {
	return func(lo *launchOptions) error {
		lo.ShellPath = s
		return nil
	}
}

// OptCwdPath specifies the initial working directory in the container.
func OptCwdPath(p string) Option {
	return func(lo *launchOptions) error {
		lo.CwdPath = p
		return nil
	}
}

// OptFakeroot enables the fake root mode, using user namespaces and subuid / subgid mapping.
func OptFakeroot(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Fakeroot = b
		return nil
	}
}

// OptBoot enables execution of /sbin/init on startup of an instance container.
func OptBoot(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Boot = b
		return nil
	}
}

// OptNoInit disables shim process when PID namespace is used.
func OptNoInit(b bool) Option {
	return func(lo *launchOptions) error {
		lo.NoInit = b
		return nil
	}
}

// OptContain starts the container with minimal /dev and empty home/tmp mounts.
func OptContain(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Contain = b
		return nil
	}
}

// OptContainAll infers Contain, and adds PID, IPC namespaces, and CleanEnv.
func OptContainAll(b bool) Option {
	return func(lo *launchOptions) error {
		lo.ContainAll = b
		return nil
	}
}

// OptAppName sets a SCIF application name to run.
func OptAppName(a string) Option {
	return func(lo *launchOptions) error {
		lo.AppName = a
		return nil
	}
}

// OptKeyInfo sets encryption key material to use when accessing an encrypted container image.
func OptKeyInfo(ki *cryptkey.KeyInfo) Option {
	return func(lo *launchOptions) error {
		lo.KeyInfo = ki
		return nil
	}
}

// CacheDisabled indicates caching of images was disabled in the CLI.
func OptCacheDisabled(b bool) Option {
	return func(lo *launchOptions) error {
		lo.CacheDisabled = b
		return nil
	}
}

// OptDMTCPLaunch
func OptDMTCPLaunch(a string) Option {
	return func(lo *launchOptions) error {
		lo.DMTCPLaunch = a
		return nil
	}
}

// OptDMTCPRestart
func OptDMTCPRestart(a string) Option {
	return func(lo *launchOptions) error {
		lo.DMTCPRestart = a
		return nil
	}
}

// OptUnsquash
func OptUnsquash(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Unsquash = b
		return nil
	}
}

// OptIgnoreSubuid
func OptIgnoreSubuid(b bool) Option {
	return func(lo *launchOptions) error {
		lo.IgnoreSubuid = b
		return nil
	}
}

// OptIgnoreFakerootCmd
func OptIgnoreFakerootCmd(b bool) Option {
	return func(lo *launchOptions) error {
		lo.IgnoreFakerootCmd = b
		return nil
	}
}

// OptIgnoreUserns
func OptIgnoreUserns(b bool) Option {
	return func(lo *launchOptions) error {
		lo.IgnoreUserns = b
		return nil
	}
}

// OptUseBuildConfig
func OptUseBuildConfig(b bool) Option {
	return func(lo *launchOptions) error {
		lo.UseBuildConfig = b
		return nil
	}
}

// OptTmpDir
func OptTmpDir(a string) Option {
	return func(lo *launchOptions) error {
		lo.TmpDir = a
		return nil
	}
}

// OptUnderlay
func OptUnderlay(b bool) Option {
	return func(lo *launchOptions) error {
		lo.Underlay = b
		return nil
	}
}

// OptShareNSMode
func OptShareNSMode(b bool) Option {
	return func(lo *launchOptions) error {
		lo.ShareNSMode = b
		return nil
	}
}

// OptShareNSFd
func OptShareNSFd(fd int) Option {
	return func(lo *launchOptions) error {
		lo.ShareNSFd = fd
		return nil
	}
}

// OptRunscriptTimeout
func OptRunscriptTimeout(timeout string) Option {
	return func(lo *launchOptions) error {
		lo.RunscriptTimeout = timeout
		return nil
	}
}

// OptDevice sets the list of fully-qualified CDI device names.
func OptDevice(devices []string) Option {
	return func(lo *launchOptions) error {
		lo.Devices = devices
		return nil
	}
}

// OptCdiDirs sets the list of directories in which CDI should look for device definition JSON files.
func OptCdiDirs(dirs []string) Option {
	return func(lo *launchOptions) error {
		lo.CdiDirs = dirs
		return nil
	}
}

// OptIntelHpu
func OptIntelHpu(b bool) Option {
	return func(lo *launchOptions) error {
		lo.IntelHpu = b
		return nil
	}
}
