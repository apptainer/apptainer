// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"os"

	"github.com/apptainer/apptainer/pkg/cmdline"
)

// actionflags.go contains flag variables for action-like commands to draw from
var (
	appName          string
	bindPaths        []string
	mounts           []string
	homePath         string
	overlayPath      []string
	scratchPath      []string
	workdirPath      string
	pwdPath          string
	shellPath        string
	hostname         string
	network          string
	networkArgs      []string
	dns              string
	security         []string
	cgroupsTOMLFile  string
	vmRAM            string
	vmCPU            string
	vmIP             string
	containLibsPath  []string
	fuseMount        []string
	apptainerEnv     map[string]string
	apptainerEnvFile string
	noMount          []string
	dmtcpLaunch      string
	dmtcpRestart     string

	isBoot          bool
	isFakeroot      bool
	isCleanEnv      bool
	isCompat        bool
	isContained     bool
	isContainAll    bool
	isWritable      bool
	isWritableTmpfs bool
	nvidia          bool
	nvCCLI          bool
	rocm            bool
	noEval          bool
	noHome          bool
	noInit          bool
	noNvidia        bool
	noRocm          bool
	noUmask         bool
	vm              bool
	vmErr           bool
	isSyOS          bool
	disableCache    bool

	netNamespace  bool
	utsNamespace  bool
	userNamespace bool
	pidNamespace  bool
	ipcNamespace  bool

	allowSUID bool
	keepPrivs bool
	noPrivs   bool
	addCaps   string
	dropCaps  string

	blkioWeight       int
	blkioWeightDevice []string
	cpuShares         int
	cpus              string // decimal
	cpuSetCPUs        string
	cpuSetMems        string
	memory            string // bytes
	memoryReservation string // bytes
	memorySwap        string // bytes
	oomKillDisable    bool
	pidsLimit         int
	unsquash          bool

	ignoreSubuid      bool
	ignoreFakerootCmd bool
	ignoreUserns      bool

	underlay bool // whether using underlay instead of overlay
)

// --app
var actionAppFlag = cmdline.Flag{
	ID:           "actionAppFlag",
	Value:        &appName,
	DefaultValue: "",
	Name:         "app",
	Usage:        "set an application to run inside a container",
	EnvKeys:      []string{"APP", "APPNAME"},
}

// -B|--bind
var actionBindFlag = cmdline.Flag{
	ID:           "actionBindFlag",
	Value:        &bindPaths,
	DefaultValue: cmdline.StringArray{}, // to allow commas in bind path
	Name:         "bind",
	ShortHand:    "B",
	Usage:        "a user-bind path specification.  spec has the format src[:dest[:opts]], where src and dest are outside and inside paths.  If dest is not given, it is set equal to src.  Mount options ('opts') may be specified as 'ro' (read-only) or 'rw' (read/write, which is the default). Multiple bind paths can be given by a comma separated list.",
	EnvKeys:      []string{"BIND", "BINDPATH"},
	Tag:          "<spec>",
	EnvHandler:   cmdline.EnvAppendValue,
}

// --mount
var actionMountFlag = cmdline.Flag{
	ID:           "actionMountFlag",
	Value:        &mounts,
	DefaultValue: cmdline.StringArray{},
	Name:         "mount",
	Usage:        "a mount specification e.g. 'type=bind,source=/opt,destination=/hostopt'.",
	EnvKeys:      []string{"MOUNT"},
	Tag:          "<spec>",
	EnvHandler:   cmdline.EnvAppendValue,
}

// -H|--home
var actionHomeFlag = cmdline.Flag{
	ID:           "actionHomeFlag",
	Value:        &homePath,
	DefaultValue: CurrentUser.HomeDir,
	Name:         "home",
	ShortHand:    "H",
	Usage:        "a home directory specification.  spec can either be a src path or src:dest pair.  src is the source path of the home directory outside the container and dest overrides the home directory within the container.",
	EnvKeys:      []string{"HOME"},
	Tag:          "<spec>",
}

// -o|--overlay
var actionOverlayFlag = cmdline.Flag{
	ID:           "actionOverlayFlag",
	Value:        &overlayPath,
	DefaultValue: []string{},
	Name:         "overlay",
	ShortHand:    "o",
	Usage:        "use an overlayFS image for persistent data storage or as read-only layer of container",
	EnvKeys:      []string{"OVERLAY", "OVERLAYIMAGE"},
	Tag:          "<path>",
}

// -S|--scratch
var actionScratchFlag = cmdline.Flag{
	ID:           "actionScratchFlag",
	Value:        &scratchPath,
	DefaultValue: []string{},
	Name:         "scratch",
	ShortHand:    "S",
	Usage:        "include a scratch directory within the container that is linked to a temporary dir (use -W to force location)",
	EnvKeys:      []string{"SCRATCH", "SCRATCHDIR"},
	Tag:          "<path>",
}

// -W|--workdir
var actionWorkdirFlag = cmdline.Flag{
	ID:           "actionWorkdirFlag",
	Value:        &workdirPath,
	DefaultValue: "",
	Name:         "workdir",
	ShortHand:    "W",
	Usage:        "working directory to be used for /tmp, /var/tmp and $HOME (if -c/--contain was also used)",
	EnvKeys:      []string{"WORKDIR"},
	Tag:          "<path>",
}

// --disable-cache
var actionDisableCacheFlag = cmdline.Flag{
	ID:           "actionDisableCacheFlag",
	Value:        &disableCache,
	DefaultValue: false,
	Name:         "disable-cache",
	Usage:        "do not use or create cache",
	EnvKeys:      []string{"DISABLE_CACHE"},
}

// -s|--shell
var actionShellFlag = cmdline.Flag{
	ID:           "actionShellFlag",
	Value:        &shellPath,
	DefaultValue: "",
	Name:         "shell",
	ShortHand:    "s",
	Usage:        "path to program to use for interactive shell",
	EnvKeys:      []string{"SHELL"},
	Tag:          "<path>",
}

// --pwd
var actionPwdFlag = cmdline.Flag{
	ID:           "actionPwdFlag",
	Value:        &pwdPath,
	DefaultValue: "",
	Name:         "pwd",
	Usage:        "initial working directory for payload process inside the container",
	EnvKeys:      []string{"PWD", "TARGET_PWD"},
	Tag:          "<path>",
}

// --hostname
var actionHostnameFlag = cmdline.Flag{
	ID:           "actionHostnameFlag",
	Value:        &hostname,
	DefaultValue: "",
	Name:         "hostname",
	Usage:        "set container hostname",
	EnvKeys:      []string{"HOSTNAME"},
	Tag:          "<name>",
}

// --network
var actionNetworkFlag = cmdline.Flag{
	ID:           "actionNetworkFlag",
	Value:        &network,
	DefaultValue: "",
	Name:         "network",
	Usage:        "specify desired network type separated by commas, each network will bring up a dedicated interface inside container",
	EnvKeys:      []string{"NETWORK"},
	Tag:          "<name>",
}

// --network-args
var actionNetworkArgsFlag = cmdline.Flag{
	ID:           "actionNetworkArgsFlag",
	Value:        &networkArgs,
	DefaultValue: []string{},
	Name:         "network-args",
	Usage:        "specify network arguments to pass to CNI plugins",
	EnvKeys:      []string{"NETWORK_ARGS"},
	Tag:          "<args>",
}

// --dns
var actionDNSFlag = cmdline.Flag{
	ID:           "actionDnsFlag",
	Value:        &dns,
	DefaultValue: "",
	Name:         "dns",
	Usage:        "list of DNS server separated by commas to add in resolv.conf",
	EnvKeys:      []string{"DNS"},
}

// --security
var actionSecurityFlag = cmdline.Flag{
	ID:           "actionSecurityFlag",
	Value:        &security,
	DefaultValue: []string{},
	Name:         "security",
	Usage:        "enable security features (SELinux, Apparmor, Seccomp)",
	EnvKeys:      []string{"SECURITY"},
}

// --apply-cgroups
var actionApplyCgroupsFlag = cmdline.Flag{
	ID:           "actionApplyCgroupsFlag",
	Value:        &cgroupsTOMLFile,
	DefaultValue: "",
	Name:         "apply-cgroups",
	Usage:        "apply cgroups from file for container processes (root only)",
	EnvKeys:      []string{"APPLY_CGROUPS"},
}

// --vm-ram
var actionVMRAMFlag = cmdline.Flag{
	ID:           "actionVMRAMFlag",
	Value:        &vmRAM,
	DefaultValue: "1024",
	Name:         "vm-ram",
	Usage:        "amount of RAM in MiB to allocate to Virtual Machine (implies --vm)",
	Tag:          "<size>",
	EnvKeys:      []string{"VM_RAM"},
}

// --vm-cpu
var actionVMCPUFlag = cmdline.Flag{
	ID:           "actionVMCPUFlag",
	Value:        &vmCPU,
	DefaultValue: "1",
	Name:         "vm-cpu",
	Usage:        "number of CPU cores to allocate to Virtual Machine (implies --vm)",
	Tag:          "<CPU #>",
	EnvKeys:      []string{"VM_CPU"},
}

// --vm-ip
var actionVMIPFlag = cmdline.Flag{
	ID:           "actionVMIPFlag",
	Value:        &vmIP,
	DefaultValue: "dhcp",
	Name:         "vm-ip",
	Usage:        "IP Address to assign for container usage. Defaults to DHCP within bridge network.",
	Tag:          "<IP Address>",
	EnvKeys:      []string{"VM_IP"},
}

// hidden flag to handle APPTAINER_CONTAINLIBS environment variable
var actionContainLibsFlag = cmdline.Flag{
	ID:           "actionContainLibsFlag",
	Value:        &containLibsPath,
	DefaultValue: []string{},
	Name:         "containlibs",
	Hidden:       true,
	EnvKeys:      []string{"CONTAINLIBS"},
}

// --fusemount
var actionFuseMountFlag = cmdline.Flag{
	ID:           "actionFuseMountFlag",
	Value:        &fuseMount,
	DefaultValue: []string{},
	Name:         "fusemount",
	Usage:        "A FUSE filesystem mount specification of the form '<type>:<fuse command> <mountpoint>' - where <type> is 'container' or 'host', specifying where the mount will be performed ('container-daemon' or 'host-daemon' will run the FUSE process detached). <fuse command> is the path to the FUSE executable, plus options for the mount. <mountpoint> is the location in the container to which the FUSE mount will be attached. E.g. 'container:sshfs 10.0.0.1:/ /sshfs'. Implies --pid.",
	EnvKeys:      []string{"FUSESPEC"},
}

// hidden flag to handle APPTAINER_TMPDIR environment variable
var actionTmpDirFlag = cmdline.Flag{
	ID:           "actionTmpDirFlag",
	Value:        &tmpDir,
	DefaultValue: os.TempDir(),
	Name:         "tmpdir",
	Usage:        "specify a temporary directory to use for build",
	Hidden:       true,
	EnvKeys:      []string{"TMPDIR"},
}

// --boot
var actionBootFlag = cmdline.Flag{
	ID:           "actionBootFlag",
	Value:        &isBoot,
	DefaultValue: false,
	Name:         "boot",
	Usage:        "execute /sbin/init to boot container (root only)",
	EnvKeys:      []string{"BOOT"},
}

// -f|--fakeroot
var actionFakerootFlag = cmdline.Flag{
	ID:           "actionFakerootFlag",
	Value:        &isFakeroot,
	DefaultValue: false,
	Name:         "fakeroot",
	ShortHand:    "f",
	Usage:        "run container with the appearance of running as root",
	EnvKeys:      []string{"FAKEROOT"},
}

// -e|--cleanenv
var actionCleanEnvFlag = cmdline.Flag{
	ID:           "actionCleanEnvFlag",
	Value:        &isCleanEnv,
	DefaultValue: false,
	Name:         "cleanenv",
	ShortHand:    "e",
	Usage:        "clean environment before running container",
	EnvKeys:      []string{"CLEANENV"},
}

// --compat
var actionCompatFlag = cmdline.Flag{
	ID:           "actionCompatFlag",
	Value:        &isCompat,
	DefaultValue: false,
	Name:         "compat",
	Usage:        "apply settings for increased OCI/Docker compatibility. Infers --containall, --no-init, --no-umask, --no-eval, --writable-tmpfs.",
	EnvKeys:      []string{"COMPAT"},
}

// -c|--contain
var actionContainFlag = cmdline.Flag{
	ID:           "actionContainFlag",
	Value:        &isContained,
	DefaultValue: false,
	Name:         "contain",
	ShortHand:    "c",
	Usage:        "use minimal /dev and empty other directories (e.g. /tmp and $HOME) instead of sharing filesystems from your host",
	EnvKeys:      []string{"CONTAIN"},
}

// -C|--containall
var actionContainAllFlag = cmdline.Flag{
	ID:           "actionContainAllFlag",
	Value:        &isContainAll,
	DefaultValue: false,
	Name:         "containall",
	ShortHand:    "C",
	Usage:        "contain not only file systems, but also PID, IPC, and environment",
	EnvKeys:      []string{"CONTAINALL"},
}

// --nv
var actionNvidiaFlag = cmdline.Flag{
	ID:           "actionNvidiaFlag",
	Value:        &nvidia,
	DefaultValue: false,
	Name:         "nv",
	Usage:        "enable Nvidia support",
	EnvKeys:      []string{"NV"},
}

// --nvccli
var actionNvCCLIFlag = cmdline.Flag{
	ID:           "actionNvCCLIFlag",
	Value:        &nvCCLI,
	DefaultValue: false,
	Name:         "nvccli",
	Usage:        "use nvidia-container-cli for GPU setup (experimental)",
	EnvKeys:      []string{"NVCCLI"},
}

// --rocm flag to automatically bind
var actionRocmFlag = cmdline.Flag{
	ID:           "actionRocmFlag",
	Value:        &rocm,
	DefaultValue: false,
	Name:         "rocm",
	Usage:        "enable experimental Rocm support",
	EnvKeys:      []string{"ROCM"},
}

// -w|--writable
var actionWritableFlag = cmdline.Flag{
	ID:           "actionWritableFlag",
	Value:        &isWritable,
	DefaultValue: false,
	Name:         "writable",
	ShortHand:    "w",
	Usage:        "by default all Apptainer containers are available as read only. This option makes the file system accessible as read/write.",
	EnvKeys:      []string{"WRITABLE"},
}

// --writable-tmpfs
var actionWritableTmpfsFlag = cmdline.Flag{
	ID:           "actionWritableTmpfsFlag",
	Value:        &isWritableTmpfs,
	DefaultValue: false,
	Name:         "writable-tmpfs",
	Usage:        "makes the file system accessible as read-write with non persistent data (with overlay support only)",
	EnvKeys:      []string{"WRITABLE_TMPFS"},
}

// --no-home
var actionNoHomeFlag = cmdline.Flag{
	ID:           "actionNoHomeFlag",
	Value:        &noHome,
	DefaultValue: false,
	Name:         "no-home",
	Usage:        "do NOT mount users home directory if /home is not the current working directory",
	EnvKeys:      []string{"NO_HOME"},
}

// --no-mount
var actionNoMountFlag = cmdline.Flag{
	ID:           "actionNoMountFlag",
	Value:        &noMount,
	DefaultValue: []string{},
	Name:         "no-mount",
	Usage:        "disable one or more 'mount xxx' options set in apptainer.conf and/or specify absolute destination path to disable a bind path entry, or 'bind-paths' to disable all bind path entries.",
	EnvKeys:      []string{"NO_MOUNT"},
}

// --no-init
var actionNoInitFlag = cmdline.Flag{
	ID:           "actionNoInitFlag",
	Value:        &noInit,
	DefaultValue: false,
	Name:         "no-init",
	Usage:        "do NOT start shim process with --pid",
	EnvKeys:      []string{"NOSHIMINIT"},
}

// hidden flag to disable nvidia bindings when 'always use nv = yes'
var actionNoNvidiaFlag = cmdline.Flag{
	ID:           "actionNoNvidiaFlag",
	Value:        &noNvidia,
	DefaultValue: false,
	Name:         "no-nv",
	Hidden:       true,
	EnvKeys:      []string{"NV_OFF", "NO_NV"},
}

// hidden flag to disable rocm bindings when 'always use rocm = yes'
var actionNoRocmFlag = cmdline.Flag{
	ID:           "actionNoRocmFlag",
	Value:        &noRocm,
	DefaultValue: false,
	Name:         "no-rocm",
	Hidden:       true,
	EnvKeys:      []string{"ROCM_OFF", "NO_ROCM"},
}

// --vm
var actionVMFlag = cmdline.Flag{
	ID:           "actionVMFlag",
	Value:        &vm,
	DefaultValue: false,
	Name:         "vm",
	Usage:        "enable VM support",
	EnvKeys:      []string{"VM"},
}

// --vm-err
var actionVMErrFlag = cmdline.Flag{
	ID:           "actionVMErrFlag",
	Value:        &vmErr,
	DefaultValue: false,
	Name:         "vm-err",
	Usage:        "enable attaching stderr from VM",
	EnvKeys:      []string{"VMERROR"},
}

// --syos
// TODO: Keep this in production?
var actionSyOSFlag = cmdline.Flag{
	ID:           "actionSyOSFlag",
	Value:        &isSyOS,
	DefaultValue: false,
	Name:         "syos",
	Usage:        "execute SyOS shell",
	EnvKeys:      []string{"SYOS"},
}

// -p|--pid
var actionPidNamespaceFlag = cmdline.Flag{
	ID:           "actionPidNamespaceFlag",
	Value:        &pidNamespace,
	DefaultValue: false,
	Name:         "pid",
	ShortHand:    "p",
	Usage:        "run container in a new PID namespace",
	EnvKeys:      []string{"PID", "UNSHARE_PID"},
}

// -i|--ipc
var actionIpcNamespaceFlag = cmdline.Flag{
	ID:           "actionIpcNamespaceFlag",
	Value:        &ipcNamespace,
	DefaultValue: false,
	Name:         "ipc",
	ShortHand:    "i",
	Usage:        "run container in a new IPC namespace",
	EnvKeys:      []string{"IPC", "UNSHARE_IPC"},
}

// -n|--net
var actionNetNamespaceFlag = cmdline.Flag{
	ID:           "actionNetNamespaceFlag",
	Value:        &netNamespace,
	DefaultValue: false,
	Name:         "net",
	ShortHand:    "n",
	Usage:        "run container in a new network namespace (sets up a bridge network interface by default)",
	EnvKeys:      []string{"NET", "UNSHARE_NET"},
}

// --uts
var actionUtsNamespaceFlag = cmdline.Flag{
	ID:           "actionUtsNamespaceFlag",
	Value:        &utsNamespace,
	DefaultValue: false,
	Name:         "uts",
	Usage:        "run container in a new UTS namespace",
	EnvKeys:      []string{"UTS", "UNSHARE_UTS"},
}

// -u|--userns
var actionUserNamespaceFlag = cmdline.Flag{
	ID:           "actionUserNamespaceFlag",
	Value:        &userNamespace,
	DefaultValue: false,
	Name:         "userns",
	ShortHand:    "u",
	Usage:        "run container in a new user namespace",
	EnvKeys:      []string{"USERNS", "UNSHARE_USERNS"},
}

// --keep-privs
var actionKeepPrivsFlag = cmdline.Flag{
	ID:           "actionKeepPrivsFlag",
	Value:        &keepPrivs,
	DefaultValue: false,
	Name:         "keep-privs",
	Usage:        "let root user keep privileges in container (root only)",
	EnvKeys:      []string{"KEEP_PRIVS"},
}

// --no-privs
var actionNoPrivsFlag = cmdline.Flag{
	ID:           "actionNoPrivsFlag",
	Value:        &noPrivs,
	DefaultValue: false,
	Name:         "no-privs",
	Usage:        "drop all privileges from root user in container)",
	EnvKeys:      []string{"NO_PRIVS"},
}

// --add-caps
var actionAddCapsFlag = cmdline.Flag{
	ID:           "actionAddCapsFlag",
	Value:        &addCaps,
	DefaultValue: "",
	Name:         "add-caps",
	Usage:        "a comma separated capability list to add",
	EnvKeys:      []string{"ADD_CAPS"},
}

// --drop-caps
var actionDropCapsFlag = cmdline.Flag{
	ID:           "actionDropCapsFlag",
	Value:        &dropCaps,
	DefaultValue: "",
	Name:         "drop-caps",
	Usage:        "a comma separated capability list to drop",
	EnvKeys:      []string{"DROP_CAPS"},
}

// --allow-setuid
var actionAllowSetuidFlag = cmdline.Flag{
	ID:           "actionAllowSetuidFlag",
	Value:        &allowSUID,
	DefaultValue: false,
	Name:         "allow-setuid",
	Usage:        "allow setuid binaries in container (root only)",
	EnvKeys:      []string{"ALLOW_SETUID"},
}

// --env
var actionEnvFlag = cmdline.Flag{
	ID:           "actionEnvFlag",
	Value:        &apptainerEnv,
	DefaultValue: map[string]string{},
	Name:         "env",
	Usage:        "pass environment variable to contained process",
}

// --env-file
var actionEnvFileFlag = cmdline.Flag{
	ID:           "actionEnvFileFlag",
	Value:        &apptainerEnvFile,
	DefaultValue: "",
	Name:         "env-file",
	Usage:        "pass environment variables from file to contained process",
	EnvKeys:      []string{"ENV_FILE"},
}

// --no-umask
var actionNoUmaskFlag = cmdline.Flag{
	ID:           "actionNoUmask",
	Value:        &noUmask,
	DefaultValue: false,
	Name:         "no-umask",
	Usage:        "do not propagate umask to the container, set default 0022 umask",
	EnvKeys:      []string{"NO_UMASK"},
}

// --no-eval
var actionNoEvalFlag = cmdline.Flag{
	ID:           "actionNoEval",
	Value:        &noEval,
	DefaultValue: false,
	Name:         "no-eval",
	Usage:        "do not shell evaluate env vars or OCI container CMD/ENTRYPOINT/ARGS",
	EnvKeys:      []string{"NO_EVAL"},
}

// --dmtcp-launch
var actionDMTCPLaunchFlag = cmdline.Flag{
	ID:           "actionDMTCPLaunchFlag",
	Value:        &dmtcpLaunch,
	DefaultValue: "",
	Name:         "dmtcp-launch",
	Usage:        "checkpoint for dmtcp to save container process state to (experimental)",
	EnvKeys:      []string{"DMTCP_LAUNCH"},
}

// --dmtcp-restart
var actionDMTCPRestartFlag = cmdline.Flag{
	ID:           "actionDMTCPrestartFlag",
	Value:        &dmtcpRestart,
	DefaultValue: "",
	Name:         "dmtcp-restart",
	Usage:        "checkpoint for dmtcp to use to restart container process (experimental)",
	EnvKeys:      []string{"DMTCP_RESTART"},
}

// --blkio-weight
var actionBlkioWeightFlag = cmdline.Flag{
	ID:           "actionBlkioWeight",
	Value:        &blkioWeight,
	DefaultValue: 0,
	Name:         "blkio-weight",
	Usage:        "Block IO relative weight in range 10-1000, 0 to disable",
	EnvKeys:      []string{"BLKIO_WEIGHT"},
}

// --blkio-weight-device
var actionBlkioWeightDeviceFlag = cmdline.Flag{
	ID:           "actionBlkioWeightDevice",
	Value:        &blkioWeightDevice,
	DefaultValue: []string{},
	Name:         "blkio-weight-device",
	Usage:        "Device specific block IO relative weight",
	EnvKeys:      []string{"BLKIO_WEIGHT_DEVICE"},
}

// --cpu-shares
var actionCPUSharesFlag = cmdline.Flag{
	ID:           "actionCPUShares",
	Value:        &cpuShares,
	DefaultValue: -1,
	Name:         "cpu-shares",
	Usage:        "CPU shares for container",
	EnvKeys:      []string{"CPU_SHARES"},
}

// --cpus
var actionCPUsFlag = cmdline.Flag{
	ID:           "actionCPUs",
	Value:        &cpus,
	DefaultValue: "",
	Name:         "cpus",
	Usage:        "Number of CPUs available to container",
	EnvKeys:      []string{"CPU_SHARES"},
}

// --cpuset-cpus
var actionCPUsetCPUsFlag = cmdline.Flag{
	ID:           "actionCPUsetCPUs",
	Value:        &cpuSetCPUs,
	DefaultValue: "",
	Name:         "cpuset-cpus",
	Usage:        "List of host CPUs available to container",
	EnvKeys:      []string{"CPUSET_CPUS"},
}

// --cpuset-mems
var actionCPUsetMemsFlag = cmdline.Flag{
	ID:           "actionCPUsetMems",
	Value:        &cpuSetMems,
	DefaultValue: "",
	Name:         "cpuset-mems",
	Usage:        "List of host memory nodes available to container",
	EnvKeys:      []string{"CPUSET_MEMS"},
}

// --memory
var actionMemoryFlag = cmdline.Flag{
	ID:           "actionMemory",
	Value:        &memory,
	DefaultValue: "",
	Name:         "memory",
	Usage:        "Memory limit in bytes",
	EnvKeys:      []string{"MEMORY"},
}

// --memory-reservation
var actionMemoryReservationFlag = cmdline.Flag{
	ID:           "actionMemoryReservation",
	Value:        &memoryReservation,
	DefaultValue: "",
	Name:         "memory-reservation",
	Usage:        "Memory soft limit in bytes",
	EnvKeys:      []string{"MEMORY_RESERVATION"},
}

// --memory-swap
var actionMemorySwapFlag = cmdline.Flag{
	ID:           "actionMemorySwap",
	Value:        &memorySwap,
	DefaultValue: "",
	Name:         "memory-swap",
	Usage:        "Swap limit, use -1 for unlimited swap",
	EnvKeys:      []string{"MEMORY_SWAP"},
}

// --oom-kill-disable
var actionOomKillDisableFlag = cmdline.Flag{
	ID:           "oomKillDisable",
	Value:        &oomKillDisable,
	DefaultValue: false,
	Name:         "oom-kill-disable",
	Usage:        "Disable OOM killer",
	EnvKeys:      []string{"OOM_KILL_DISABLE"},
}

// --pids-limit
var actionPidsLimitFlag = cmdline.Flag{
	ID:           "actionPidsLimit",
	Value:        &pidsLimit,
	DefaultValue: 0,
	Name:         "pids-limit",
	Usage:        "Limit number of container PIDs, use -1 for unlimited",
	EnvKeys:      []string{"PIDS_LIMIT"},
}

// --unsquash
var actionUnsquashFlag = cmdline.Flag{
	ID:           "actionUnsquashFlag",
	Value:        &unsquash,
	DefaultValue: false,
	Name:         "unsquash",
	Usage:        "Convert SIF file to temporary sandbox before running",
	EnvKeys:      []string{"UNSQUASH"},
}

// --ignore-subuid
var actionIgnoreSubuidFlag = cmdline.Flag{
	ID:           "actionIgnoreSubuidFlag",
	Value:        &ignoreSubuid,
	DefaultValue: false,
	Name:         "ignore-subuid",
	Usage:        "ignore entries inside /etc/subuid",
	EnvKeys:      []string{"IGNORE_SUBUID"},
	Hidden:       true,
}

// --ignore-fakeroot-command
var actionIgnoreFakerootCommand = cmdline.Flag{
	ID:           "actionIgnoreFakerootCommandFlag",
	Value:        &ignoreFakerootCmd,
	DefaultValue: false,
	Name:         "ignore-fakeroot-command",
	Usage:        "ignore fakeroot command",
	EnvKeys:      []string{"IGNORE_FAKEROOT_COMMAND"},
	Hidden:       true,
}

// --ignore-userns
var actionIgnoreUsernsFlag = cmdline.Flag{
	ID:           "actionIgnoreUsernsFlag",
	Value:        &ignoreUserns,
	DefaultValue: false,
	Name:         "ignore-userns",
	Usage:        "ignore user namespaces",
	EnvKeys:      []string{"IGNORE_USERNS"},
	Hidden:       true,
}

// --underlay
var actionUnderlayFlag = cmdline.Flag{
	ID:           "underlayFlag",
	Value:        &underlay,
	DefaultValue: false,
	Name:         "underlay",
	Usage:        "use underlay",
	EnvKeys:      []string{"UNDERLAY"},
	Hidden:       false,
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(ExecCmd)
		cmdManager.RegisterCmd(ShellCmd)
		cmdManager.RegisterCmd(RunCmd)
		cmdManager.RegisterCmd(TestCmd)

		cmdManager.SetCmdGroup("actions", ExecCmd, ShellCmd, RunCmd, TestCmd)
		actionsCmd := cmdManager.GetCmdGroup("actions")

		if instanceStartCmd != nil {
			cmdManager.SetCmdGroup("actions_instance", ExecCmd, ShellCmd, RunCmd, TestCmd, instanceStartCmd, instanceRunCmd)
			cmdManager.RegisterFlagForCmd(&actionBootFlag, instanceStartCmd, instanceRunCmd)
		} else {
			cmdManager.SetCmdGroup("actions_instance", actionsCmd...)
		}
		actionsInstanceCmd := cmdManager.GetCmdGroup("actions_instance")

		cmdManager.RegisterFlagForCmd(&actionAddCapsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionAllowSetuidFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionAppFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionApplyCgroupsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionBindFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionCleanEnvFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionCompatFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionContainAllFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionContainFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionContainLibsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionDisableCacheFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionDNSFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionDropCapsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionFakerootFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionFuseMountFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionHomeFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionHostnameFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionIpcNamespaceFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionKeepPrivsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionMountFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNetNamespaceFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNetworkArgsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNetworkFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoHomeFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoMountFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoInitFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoNvidiaFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoRocmFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoPrivsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNvidiaFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNvCCLIFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionRocmFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionOverlayFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&commonPromptForPassphraseFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&commonPEMFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionPidNamespaceFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionPwdFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionScratchFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionSecurityFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionShellFlag, ShellCmd)
		cmdManager.RegisterFlagForCmd(&actionSyOSFlag, ShellCmd)
		cmdManager.RegisterFlagForCmd(&actionTmpDirFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionUserNamespaceFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionUtsNamespaceFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionVMCPUFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionVMErrFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionVMFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionVMIPFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionVMRAMFlag, actionsCmd...)
		cmdManager.RegisterFlagForCmd(&actionWorkdirFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionWritableFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionWritableTmpfsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&commonNoHTTPSFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&commonOldNoHTTPSFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&dockerLoginFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&dockerHostFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&dockerPasswordFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&dockerUsernameFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionEnvFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionEnvFileFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoUmaskFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionNoEvalFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionBlkioWeightFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionBlkioWeightDeviceFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionCPUSharesFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionCPUsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionCPUsetCPUsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionCPUsetMemsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionMemoryFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionMemoryReservationFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionMemorySwapFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionOomKillDisableFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionPidsLimitFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionUnsquashFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionIgnoreSubuidFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionIgnoreFakerootCommand, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionIgnoreUsernsFlag, actionsInstanceCmd...)
		cmdManager.RegisterFlagForCmd(&actionUnderlayFlag, actionsInstanceCmd...)
	})
}
