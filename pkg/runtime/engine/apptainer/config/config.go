// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

// Name is the name of the runtime.
const Name = "apptainer"

const (
	// DefaultLayer is the string representation for the default layer.
	DefaultLayer string = "none"
	// OverlayLayer is the string representation for the overlay layer.
	OverlayLayer = "overlay"
	// UnderlayLayer is the string representation for the underlay layer.
	UnderlayLayer = "underlay"
)

// EngineConfig stores the JSONConfig, the OciConfig and the File configuration.
type EngineConfig struct {
	JSON      *JSONConfig         `json:"jsonConfig"`
	OciConfig *oci.Config         `json:"ociConfig"`
	File      *apptainerconf.File `json:"fileConfig"`
}

// NewConfig returns apptainer.EngineConfig.
func NewConfig() *EngineConfig {
	ret := &EngineConfig{
		JSON:      new(JSONConfig),
		OciConfig: new(oci.Config),
		File:      new(apptainerconf.File),
	}
	return ret
}

// FuseMount stores the FUSE-related information required or provided by
// plugins implementing options to add FUSE filesystems in the
// container.
type FuseMount struct {
	Program       []string  `json:"program,omitempty"`       // the FUSE driver program and all required arguments
	MountPoint    string    `json:"mountPoint,omitempty"`    // the mount point for the FUSE filesystem
	Fd            int       `json:"fd,omitempty"`            // /dev/fuse file descriptor
	FromContainer bool      `json:"fromContainer,omitempty"` // is FUSE driver program is run from container or from host
	Daemon        bool      `json:"daemon,omitempty"`        // is FUSE driver program is run in daemon/background mode
	Cmd           *exec.Cmd `json:"-"`                       // holds the process exec command when FUSE driver run in foreground mode
}

// DMTCPConfig stores the DMTCP-related information required for
// container process checkpoint/restart behavior.
type DMTCPConfig struct {
	Enabled    bool     `json:"enabled,omitempty"`
	Restart    bool     `json:"restart,omitempty"`
	Checkpoint string   `json:"checkpoint,omitempty"`
	Args       []string `json:"args,omitempty"`
}

type UserInfo struct {
	Username string         `json:"username,omitempty"`
	Home     string         `json:"home,omitempty"`
	UID      int            `json:"uid,omitempty"`
	GID      int            `json:"gid,omitempty"`
	Groups   map[int]string `json:"groups,omitempty"`
	Gecos    string         `json:"gecos,omitempty"`
	Shell    string         `json:"shell,omitempty"`
}

// JSONConfig stores engine specific configuration that is allowed to be set by the user.
type JSONConfig struct {
	ScratchDir            []string          `json:"scratchdir,omitempty"`
	OverlayImage          []string          `json:"overlayImage,omitempty"`
	NetworkArgs           []string          `json:"networkArgs,omitempty"`
	Security              []string          `json:"security,omitempty"`
	FilesPath             []string          `json:"filesPath,omitempty"`
	LibrariesPath         []string          `json:"librariesPath,omitempty"`
	FuseMount             []FuseMount       `json:"fuseMount,omitempty"`
	ImageList             []image.Image     `json:"imageList,omitempty"`
	BindPath              []BindPath        `json:"bindpath,omitempty"`
	ApptainerEnv          map[string]string `json:"apptainerEnv,omitempty"`
	UnixSocketPair        [2]int            `json:"unixSocketPair,omitempty"`
	OpenFd                []int             `json:"openFd,omitempty"`
	TargetGID             []int             `json:"targetGID,omitempty"`
	Image                 string            `json:"image"`
	ImageArg              string            `json:"imageArg"`
	Workdir               string            `json:"workdir,omitempty"`
	ConfigDir             string            `json:"configdir,omitempty"`
	CgroupsJSON           string            `json:"cgroupsJSON,omitempty"`
	HomeSource            string            `json:"homedir,omitempty"`
	HomeDest              string            `json:"homeDest,omitempty"`
	Command               string            `json:"command,omitempty"`
	Shell                 string            `json:"shell,omitempty"`
	FakerootPath          string            `json:"fakerootPath,omitempty"`
	TmpDir                string            `json:"tmpdir,omitempty"`
	AddCaps               string            `json:"addCaps,omitempty"`
	DropCaps              string            `json:"dropCaps,omitempty"`
	Hostname              string            `json:"hostname,omitempty"`
	Network               string            `json:"network,omitempty"`
	DNS                   string            `json:"dns,omitempty"`
	Cwd                   string            `json:"cwd,omitempty"`
	SessionLayer          string            `json:"sessionLayer,omitempty"`
	ConfigurationFile     string            `json:"configurationFile,omitempty"`
	UseBuildConfig        bool              `json:"useBuildConfig,omitempty"`
	EncryptionKey         []byte            `json:"encryptionKey,omitempty"`
	TargetUID             int               `json:"targetUID,omitempty"`
	WritableImage         bool              `json:"writableImage,omitempty"`
	WritableTmpfs         bool              `json:"writableTmpfs,omitempty"`
	Contain               bool              `json:"container,omitempty"`
	NvLegacy              bool              `json:"nvLegacy,omitempty"`
	NvCCLI                bool              `json:"nvCCLI,omitempty"`
	NvCCLIEnv             []string          `json:"NvCCLIEnv,omitempty"`
	Rocm                  bool              `json:"rocm,omitempty"`
	CustomHome            bool              `json:"customHome,omitempty"`
	Instance              bool              `json:"instance,omitempty"`
	InstanceJoin          bool              `json:"instanceJoin,omitempty"`
	BootInstance          bool              `json:"bootInstance,omitempty"`
	RunPrivileged         bool              `json:"runPrivileged,omitempty"`
	AllowSUID             bool              `json:"allowSUID,omitempty"`
	KeepPrivs             bool              `json:"keepPrivs,omitempty"`
	NoPrivs               bool              `json:"noPrivs,omitempty"`
	NoProc                bool              `json:"noProc,omitempty"`
	NoSys                 bool              `json:"noSys,omitempty"`
	NoDev                 bool              `json:"noDev,omitempty"`
	NoDevPts              bool              `json:"noDevPts,omitempty"`
	NoHome                bool              `json:"noHome,omitempty"`
	NoTmp                 bool              `json:"noTmp,omitempty"`
	NoHostfs              bool              `json:"noHostfs,omitempty"`
	NoCwd                 bool              `json:"noCwd,omitempty"`
	SkipBinds             []string          `json:"skipBinds,omitempty"`
	NoInit                bool              `json:"noInit,omitempty"`
	Fakeroot              bool              `json:"fakeroot,omitempty"`
	SignalPropagation     bool              `json:"signalPropagation,omitempty"`
	RestoreUmask          bool              `json:"restoreUmask,omitempty"`
	DeleteTempDir         string            `json:"deleteTempDir,omitempty"`
	Umask                 int               `json:"umask,omitempty"`
	DMTCPConfig           DMTCPConfig       `json:"dmtcpConfig,omitempty"`
	XdgRuntimeDir         string            `json:"xdgRuntimeDir,omitempty"`
	DbusSessionBusAddress string            `json:"dbusSessionBusAddress,omitempty"`
	NoEval                bool              `json:"noEval,omitempty"`
	Underlay              bool              `json:"underlay,omitempty"`
	UserInfo              UserInfo          `json:"userInfo,omitempty"`
	WritableOverlay       bool              `json:"writableOverlay,omitempty"`
	OverlayImplied        bool              `json:"overlayImplied,omitempty"`
	ShareNSMode           bool              `json:"sharensMode,omitempty"`
	ShareNSFd             int               `json:"sharensFd,omitempty"`
	RunscriptTimeout      string            `json:"runscriptTimeout,omitempty"`
}

// SetImage sets the container image path to be used by EngineConfig.JSON.
func (e *EngineConfig) SetImage(name string) {
	e.JSON.Image = name
}

// GetImage retrieves the container image path.
func (e *EngineConfig) GetImage() string {
	return e.JSON.Image
}

// SetImageArg sets the container image argument to be used by EngineConfig.JSON.
func (e *EngineConfig) SetImageArg(name string) {
	e.JSON.ImageArg = name
}

// GetImageArg retrieves the container image argument.
func (e *EngineConfig) GetImageArg() string {
	return e.JSON.ImageArg
}

// SetEncryptionKey sets the key for the image's system partition.
func (e *EngineConfig) SetEncryptionKey(key []byte) {
	e.JSON.EncryptionKey = key
}

// GetEncryptionKey retrieves the key for image's system partition.
func (e *EngineConfig) GetEncryptionKey() []byte {
	return e.JSON.EncryptionKey
}

// SetWritableImage defines the container image as writable or not.
func (e *EngineConfig) SetWritableImage(writable bool) {
	e.JSON.WritableImage = writable
}

// GetWritableImage returns if the container image is writable or not.
func (e *EngineConfig) GetWritableImage() bool {
	return e.JSON.WritableImage
}

// SetOverlayImage sets the overlay image path to be used on top of container image.
func (e *EngineConfig) SetOverlayImage(paths []string) {
	e.JSON.OverlayImage = paths
}

// GetOverlayImage retrieves the overlay image path.
func (e *EngineConfig) GetOverlayImage() []string {
	return e.JSON.OverlayImage
}

// SetContain sets contain flag.
func (e *EngineConfig) SetContain(contain bool) {
	e.JSON.Contain = contain
}

// GetContain returns if contain flag is set or not.
func (e *EngineConfig) GetContain() bool {
	return e.JSON.Contain
}

// SetNvLegacy sets nvLegacy flag to bind cuda libraries into containee.JSON.
func (e *EngineConfig) SetNvLegacy(nv bool) {
	e.JSON.NvLegacy = nv
}

// GetNvLegacy returns if nv flag is set or not.
func (e *EngineConfig) GetNvLegacy() bool {
	return e.JSON.NvLegacy
}

// SetNvCCLI sets nvcontainer flag to use nvidia-container-cli for CUDA setup
func (e *EngineConfig) SetNvCCLI(nvCCLI bool) {
	e.JSON.NvCCLI = nvCCLI
}

// GetNvCCLI returns if NvCCLI flag is set or not.
func (e *EngineConfig) GetNvCCLI() bool {
	return e.JSON.NvCCLI
}

// SetNvCCLIEnv sets env vars holding options for nvidia-container-cli GPU setup
func (e *EngineConfig) SetNvCCLIEnv(NvCCLIEnv []string) {
	e.JSON.NvCCLIEnv = NvCCLIEnv
}

// GetNvCCLIEnv returns env vars holding options for nvidia-container-cli GPU setup
func (e *EngineConfig) GetNvCCLIEnv() []string {
	return e.JSON.NvCCLIEnv
}

// SetRocm sets rocm flag to bind rocm libraries into containee.JSON.
func (e *EngineConfig) SetRocm(rocm bool) {
	e.JSON.Rocm = rocm
}

// GetRocm returns if rocm flag is set or not.
func (e *EngineConfig) GetRocm() bool {
	return e.JSON.Rocm
}

// SetWorkdir sets a work directory path.
func (e *EngineConfig) SetWorkdir(name string) {
	e.JSON.Workdir = name
}

// GetWorkdir retrieves the work directory path.
func (e *EngineConfig) GetWorkdir() string {
	return e.JSON.Workdir
}

// SetConfigDir sets a config directory path.
func (e *EngineConfig) SetConfigDir(name string) {
	e.JSON.ConfigDir = name
}

// GetConfigDir retrieves the config directory path if it is set, or
// otherwise an empty string.
func (e *EngineConfig) GetConfigDir() string {
	return e.JSON.ConfigDir
}

// SetScratchDir set a scratch directory path.
func (e *EngineConfig) SetScratchDir(scratchdir []string) {
	e.JSON.ScratchDir = scratchdir
}

// GetScratchDir retrieves the scratch directory path.
func (e *EngineConfig) GetScratchDir() []string {
	return e.JSON.ScratchDir
}

// SetHomeSource sets the source home directory path.
func (e *EngineConfig) SetHomeSource(source string) {
	e.JSON.HomeSource = source
}

// GetHomeSource retrieves the source home directory path.
func (e *EngineConfig) GetHomeSource() string {
	return e.JSON.HomeSource
}

// SetHomeDest sets the container home directory path.
func (e *EngineConfig) SetHomeDest(dest string) {
	e.JSON.HomeDest = dest
}

// GetHomeDest retrieves the container home directory path.
func (e *EngineConfig) GetHomeDest() string {
	return e.JSON.HomeDest
}

// SetCustomHome sets if home path is a custom path or not.
func (e *EngineConfig) SetCustomHome(custom bool) {
	e.JSON.CustomHome = custom
}

// GetCustomHome retrieves if home path is a custom path.
func (e *EngineConfig) GetCustomHome() bool {
	return e.JSON.CustomHome
}

// SetBindPath sets the paths to bind into container.
func (e *EngineConfig) SetBindPath(bindpath []BindPath) {
	e.JSON.BindPath = bindpath
}

// GetBindPath retrieves the bind paths.
func (e *EngineConfig) GetBindPath() []BindPath {
	return e.JSON.BindPath
}

// SetCommand sets action command to execute.
func (e *EngineConfig) SetCommand(command string) {
	e.JSON.Command = command
}

// GetCommand retrieves action command.
func (e *EngineConfig) GetCommand() string {
	return e.JSON.Command
}

// SetShell sets shell to be used by shell command.
func (e *EngineConfig) SetShell(shell string) {
	e.JSON.Shell = shell
}

// GetShell retrieves shell for shell command.
func (e *EngineConfig) GetShell() string {
	return e.JSON.Shell
}

// SetFakerootPath sets the fakeroot path
func (e *EngineConfig) SetFakerootPath(fakerootPath string) {
	e.JSON.FakerootPath = fakerootPath
}

// GetFakerootPath retrieves the fakeroot path
func (e *EngineConfig) GetFakerootPath() string {
	return e.JSON.FakerootPath
}

// SetTmpDir sets temporary directory path.
func (e *EngineConfig) SetTmpDir(name string) {
	e.JSON.TmpDir = name
}

// GetTmpDir retrieves temporary directory path.
func (e *EngineConfig) GetTmpDir() string {
	return e.JSON.TmpDir
}

// SetInstance sets if container run as instance or not.
func (e *EngineConfig) SetInstance(instance bool) {
	e.JSON.Instance = instance
}

// GetInstance returns if container run as instance or not.
func (e *EngineConfig) GetInstance() bool {
	return e.JSON.Instance
}

// SetInstanceJoin sets if process joins an instance or not.
func (e *EngineConfig) SetInstanceJoin(join bool) {
	e.JSON.InstanceJoin = join
}

// GetInstanceJoin returns if process joins an instance or not.
func (e *EngineConfig) GetInstanceJoin() bool {
	return e.JSON.InstanceJoin
}

// SetBootInstance sets boot flag to execute /sbin/init as main instance process.
func (e *EngineConfig) SetBootInstance(boot bool) {
	e.JSON.BootInstance = boot
}

// GetBootInstance returns if boot flag is set or not
func (e *EngineConfig) GetBootInstance() bool {
	return e.JSON.BootInstance
}

// SetAddCaps sets bounding/effective/permitted/inheritable/ambient capabilities to add.
func (e *EngineConfig) SetAddCaps(caps string) {
	e.JSON.AddCaps = caps
}

// GetAddCaps retrieves bounding/effective/permitted/inheritable/ambient capabilities to add.
func (e *EngineConfig) GetAddCaps() string {
	return e.JSON.AddCaps
}

// SetDropCaps sets bounding/effective/permitted/inheritable/ambient capabilities to drop.
func (e *EngineConfig) SetDropCaps(caps string) {
	e.JSON.DropCaps = caps
}

// GetDropCaps retrieves bounding/effective/permitted/inheritable/ambient capabilities to drop.
func (e *EngineConfig) GetDropCaps() string {
	return e.JSON.DropCaps
}

// SetHostname sets hostname to use in containee.JSON.
func (e *EngineConfig) SetHostname(hostname string) {
	e.JSON.Hostname = hostname
}

// GetHostname retrieves hostname to use in containee.JSON.
func (e *EngineConfig) GetHostname() string {
	return e.JSON.Hostname
}

// SetAllowSUID sets allow-suid flag to allow to run setuid binary inside containee.JSON.
func (e *EngineConfig) SetAllowSUID(allow bool) {
	e.JSON.AllowSUID = allow
}

// GetAllowSUID returns true if allow-suid is set and false if not.
func (e *EngineConfig) GetAllowSUID() bool {
	return e.JSON.AllowSUID
}

// SetKeepPrivs sets keep-privs flag to allow root to retain all privileges.
func (e *EngineConfig) SetKeepPrivs(keep bool) {
	e.JSON.KeepPrivs = keep
}

// GetKeepPrivs returns if keep-privs is set or not.
func (e *EngineConfig) GetKeepPrivs() bool {
	return e.JSON.KeepPrivs
}

// SetNoPrivs sets no-privs flag to force root user to lose all privileges.
func (e *EngineConfig) SetNoPrivs(nopriv bool) {
	e.JSON.NoPrivs = nopriv
}

// GetNoPrivs returns if no-privs flag is set or not.
func (e *EngineConfig) GetNoPrivs() bool {
	return e.JSON.NoPrivs
}

// SetNoProc set flag to not mount proc directory.
func (e *EngineConfig) SetNoProc(val bool) {
	e.JSON.NoProc = val
}

// GetNoProc returns if no-proc flag is set or not.
func (e *EngineConfig) GetNoProc() bool {
	return e.JSON.NoProc
}

// SetNoSys set flag to not mount sys directory.
func (e *EngineConfig) SetNoSys(val bool) {
	e.JSON.NoSys = val
}

// GetNoSys returns if no-sys flag is set or not.
func (e *EngineConfig) GetNoSys() bool {
	return e.JSON.NoSys
}

// SetNoDev set flag to not mount dev directory.
func (e *EngineConfig) SetNoDev(val bool) {
	e.JSON.NoDev = val
}

// GetNoDev returns if no-dev flag is set or not.
func (e *EngineConfig) GetNoDev() bool {
	return e.JSON.NoDev
}

// SetNoDevPts set flag to not mount dev directory.
func (e *EngineConfig) SetNoDevPts(val bool) {
	e.JSON.NoDevPts = val
}

// GetNoDevPts returns if no-devpts flag is set or not.
func (e *EngineConfig) GetNoDevPts() bool {
	return e.JSON.NoDevPts
}

// SetNoHome set flag to not mount user home directory.
func (e *EngineConfig) SetNoHome(val bool) {
	e.JSON.NoHome = val
}

// GetNoHome returns if no-home flag is set or not.
func (e *EngineConfig) GetNoHome() bool {
	return e.JSON.NoHome
}

// SetNoTmp set flag to not mount tmp directories
func (e *EngineConfig) SetNoTmp(val bool) {
	e.JSON.NoTmp = val
}

// GetNoTmp returns if no-tmo flag is set or not.
func (e *EngineConfig) GetNoTmp() bool {
	return e.JSON.NoTmp
}

// SetNoHostfs set flag to not mount all host mounts.
func (e *EngineConfig) SetNoHostfs(val bool) {
	e.JSON.NoHostfs = val
}

// GetNoHostfs returns if no-hostfs flag is set or not.
func (e *EngineConfig) GetNoHostfs() bool {
	return e.JSON.NoHostfs
}

// SetNoCwd set flag to not mount CWD
func (e *EngineConfig) SetNoCwd(val bool) {
	e.JSON.NoCwd = val
}

// GetNoCwd returns if no-cwd flag is set or not.
func (e *EngineConfig) GetNoCwd() bool {
	return e.JSON.NoCwd
}

// SetSkipBinds sets bind paths to skip
func (e *EngineConfig) SetSkipBinds(val []string) {
	e.JSON.SkipBinds = val
}

// GetSkipBinds gets bind paths to skip
func (e *EngineConfig) GetSkipBinds() []string {
	return e.JSON.SkipBinds
}

// SetNoInit set noinit flag to not start shim init process.
func (e *EngineConfig) SetNoInit(val bool) {
	e.JSON.NoInit = val
}

// GetNoInit returns if noinit flag is set or not.
func (e *EngineConfig) GetNoInit() bool {
	return e.JSON.NoInit
}

// SetNetwork sets a list of commas separated networks to configure inside container.
func (e *EngineConfig) SetNetwork(network string) {
	e.JSON.Network = network
}

// GetNetwork retrieves a list of commas separated networks configured in container.
func (e *EngineConfig) GetNetwork() string {
	return e.JSON.Network
}

// SetNetworkArgs sets network arguments to pass to CNI plugins.
func (e *EngineConfig) SetNetworkArgs(args []string) {
	e.JSON.NetworkArgs = args
}

// GetNetworkArgs retrieves network arguments passed to CNI plugins.
func (e *EngineConfig) GetNetworkArgs() []string {
	return e.JSON.NetworkArgs
}

// SetDNS sets a commas separated list of DNS servers to add in resolv.conf.
func (e *EngineConfig) SetDNS(dns string) {
	e.JSON.DNS = dns
}

// GetDNS retrieves list of DNS servers.
func (e *EngineConfig) GetDNS() string {
	return e.JSON.DNS
}

// SetImageList sets image list containing opened images.
func (e *EngineConfig) SetImageList(list []image.Image) {
	e.JSON.ImageList = list
}

// GetImageList returns image list containing opened images.
func (e *EngineConfig) GetImageList() []image.Image {
	list := e.JSON.ImageList
	// Image objects are not fully passed between stages, reinitialize them
	for idx := range list {
		img := &list[idx]
		img.ReInit()
	}
	return list
}

// SetCwd sets current working directory.
func (e *EngineConfig) SetCwd(path string) {
	e.JSON.Cwd = path
}

// GetCwd returns current working directory.
func (e *EngineConfig) GetCwd() string {
	return e.JSON.Cwd
}

// SetOpenFd sets a list of open file descriptor.
func (e *EngineConfig) SetOpenFd(fds []int) {
	e.JSON.OpenFd = fds
}

// GetOpenFd returns the list of open file descriptor.
func (e *EngineConfig) GetOpenFd() []int {
	return e.JSON.OpenFd
}

// SetWritableTmpfs sets writable tmpfs flag.
func (e *EngineConfig) SetWritableTmpfs(writable bool) {
	e.JSON.WritableTmpfs = writable
}

// GetWritableTmpfs returns if writable tmpfs is set or no.
func (e *EngineConfig) GetWritableTmpfs() bool {
	return e.JSON.WritableTmpfs
}

// SetSecurity sets security feature arguments.
func (e *EngineConfig) SetSecurity(security []string) {
	e.JSON.Security = security
}

// GetSecurity returns security feature arguments.
func (e *EngineConfig) GetSecurity() []string {
	return e.JSON.Security
}

// SetCgroupsJSON sets cgroups configuration to apply.
func (e *EngineConfig) SetCgroupsJSON(data string) {
	e.JSON.CgroupsJSON = data
}

// GetCgroupsTOML returns cgroups configuration to apply.
func (e *EngineConfig) GetCgroupsJSON() string {
	return e.JSON.CgroupsJSON
}

// SetTargetUID sets target UID to execute the container process as user ID.
func (e *EngineConfig) SetTargetUID(uid int) {
	e.JSON.TargetUID = uid
}

// GetTargetUID returns the target UID.
func (e *EngineConfig) GetTargetUID() int {
	return e.JSON.TargetUID
}

// SetTargetGID sets target GIDs to execute container process as group IDs.
func (e *EngineConfig) SetTargetGID(gid []int) {
	e.JSON.TargetGID = gid
}

// GetTargetGID returns the target GIDs.
func (e *EngineConfig) GetTargetGID() []int {
	return e.JSON.TargetGID
}

// ConcatenateSliceDeduplicate concatenates two string slices and returns a string slice without duplicated entries.
func ConcatenateSliceDeduplicate(first []string, second []string) []string {
	dedup := make(map[string]struct{})
	for _, slice := range [][]string{first, second} {
		for _, elem := range slice {
			dedup[elem] = struct{}{}
		}
	}
	slice := make([]string, 0, len(dedup))
	for elem := range dedup {
		slice = append(slice, elem)
	}
	return slice
}

// SetLibrariesPath sets libraries to bind in container
// /.singularity.d/libs directory.
func (e *EngineConfig) SetLibrariesPath(libraries []string) {
	e.JSON.LibrariesPath = libraries
}

// AppendLibrariesPath adds libraries to bind in container
// /.singularity.d/libs directory.
func (e *EngineConfig) AppendLibrariesPath(libraries ...string) {
	e.JSON.LibrariesPath = ConcatenateSliceDeduplicate(e.JSON.LibrariesPath, libraries)
}

// GetLibrariesPath returns libraries to bind in container
// /.singularity.d/libs directory.
func (e *EngineConfig) GetLibrariesPath() []string {
	return e.JSON.LibrariesPath
}

// SetFilesPath sets files to bind in container (eg: --nv).
func (e *EngineConfig) SetFilesPath(files []string) {
	e.JSON.FilesPath = files
}

// AppendFilesPath adds files to bind in container (eg: --nv)
func (e *EngineConfig) AppendFilesPath(files ...string) {
	e.JSON.FilesPath = ConcatenateSliceDeduplicate(e.JSON.FilesPath, files)
}

// GetFilesPath returns files to bind in container (eg: --nv).
func (e *EngineConfig) GetFilesPath() []string {
	return e.JSON.FilesPath
}

// SetFakeroot sets fakeroot flag.
func (e *EngineConfig) SetFakeroot(fakeroot bool) {
	e.JSON.Fakeroot = fakeroot
}

// GetFakeroot returns if fakeroot is set or not.
func (e *EngineConfig) GetFakeroot() bool {
	return e.JSON.Fakeroot
}

// GetDeleteTempDir returns the path of the temporary directory containing the root filesystem
// which must be deleted after use. If no deletion is required, the empty string is returned.
func (e *EngineConfig) GetDeleteTempDir() string {
	return e.JSON.DeleteTempDir
}

// SetDeleteTempDir sets dir as the path of the temporary directory containing the root filesystem,
// which must be deleted after use.
func (e *EngineConfig) SetDeleteTempDir(dir string) {
	e.JSON.DeleteTempDir = dir
}

// SetSignalPropagation sets if engine must propagate signals from
// master process -> container process when PID namespace is disabled
// or from master process -> appinit process -> container
// process when PID namespace is enabled.
func (e *EngineConfig) SetSignalPropagation(propagation bool) {
	e.JSON.SignalPropagation = propagation
}

// GetSignalPropagation returns if engine propagate signals across
// processes (see SetSignalPropagation).
func (e *EngineConfig) GetSignalPropagation() bool {
	return e.JSON.SignalPropagation
}

// GetSessionLayer returns the session layer used to setup the
// container mount points.
func (e *EngineConfig) GetSessionLayer() string {
	return e.JSON.SessionLayer
}

// SetSessionLayer sets the session layer to use to setup the
// container mount points.
func (e *EngineConfig) SetSessionLayer(sessionLayer string) {
	e.JSON.SessionLayer = sessionLayer
}

// SetFuseMount takes a list of fuse mount options and sets
// fuse mount configuration accordingly.
func (e *EngineConfig) SetFuseMount(mount []string) error {
	e.JSON.FuseMount = make([]FuseMount, len(mount))

	for i, mountspec := range mount {
		words := strings.Fields(mountspec)

		if len(words) == 0 {
			continue
		} else if len(words) == 1 {
			return fmt.Errorf("no whitespace separators found in command %q", words[0])
		}

		prefix := strings.SplitN(words[0], ":", 2)[0]

		words[0] = strings.Replace(words[0], prefix+":", "", 1)

		e.JSON.FuseMount[i].Fd = -1
		e.JSON.FuseMount[i].MountPoint = words[len(words)-1]
		e.JSON.FuseMount[i].Program = words[0 : len(words)-1]

		switch prefix {
		case "container":
			e.JSON.FuseMount[i].FromContainer = true
		case "container-daemon":
			e.JSON.FuseMount[i].FromContainer = true
			e.JSON.FuseMount[i].Daemon = true
		case "host":
			e.JSON.FuseMount[i].FromContainer = false
		case "host-daemon":
			e.JSON.FuseMount[i].FromContainer = false
			e.JSON.FuseMount[i].Daemon = true
		default:
			return fmt.Errorf("fusemount spec begin with an unknown prefix %s", prefix)
		}
	}

	return nil
}

// GetFuseMount returns the list of fuse mount after processing
// by SetFuseMount.
func (e *EngineConfig) GetFuseMount() []FuseMount {
	return e.JSON.FuseMount
}

// SetUnixSocketPair sets a unix socketpair used to pass file
// descriptors between RPC and master process, actually used
// to pass /dev/fuse file descriptors.
func (e *EngineConfig) SetUnixSocketPair(fds [2]int) {
	e.JSON.UnixSocketPair = fds
}

// GetUnixSocketPair returns the unix socketpair previously set
// in stage one by the engine.
func (e *EngineConfig) GetUnixSocketPair() [2]int {
	return e.JSON.UnixSocketPair
}

// SetApptainerEnv sets apptainer environment variables
// as a key/value string map.
func (e *EngineConfig) SetApptainerEnv(senv map[string]string) {
	e.JSON.ApptainerEnv = senv
}

// GetApptainerEnv returns apptainer environment variables
// as a key/value string map.
func (e *EngineConfig) GetApptainerEnv() map[string]string {
	return e.JSON.ApptainerEnv
}

// SetConfigurationFile sets the apptainer configuration file to
// use instead of the default one.
func (e *EngineConfig) SetConfigurationFile(filename string) {
	e.JSON.ConfigurationFile = filename
}

// GetConfigurationFile returns the apptainer configuration file to use.
func (e *EngineConfig) GetConfigurationFile() string {
	return e.JSON.ConfigurationFile
}

// SetUseBuildConfig defines whether to use the build configuration or not.
func (e *EngineConfig) SetUseBuildConfig(useBuildConfig bool) {
	e.JSON.UseBuildConfig = useBuildConfig
}

// GetUseBuildConfig returns if the build configuration should be used or not.
func (e *EngineConfig) GetUseBuildConfig() bool {
	return e.JSON.UseBuildConfig
}

// SetRestoreUmask returns whether to restore Umask for the container launched process.
func (e *EngineConfig) SetRestoreUmask(restoreUmask bool) {
	e.JSON.RestoreUmask = restoreUmask
}

// GetRestoreUmask returns the umask to be used in the container launched process.
func (e *EngineConfig) GetRestoreUmask() bool {
	return e.JSON.RestoreUmask
}

// SetUmask sets the umask to be used in the container launched process.
func (e *EngineConfig) SetUmask(umask int) {
	e.JSON.Umask = umask
}

// GetUmask returns the umask to be used in the container launched process.
func (e *EngineConfig) GetUmask() int {
	return e.JSON.Umask
}

// SetDMTCPConfig sets the dmtcp configuration for the engine to used for the container process.
func (e *EngineConfig) SetDMTCPConfig(config DMTCPConfig) {
	e.JSON.DMTCPConfig = config
}

// GetDMTCPConfig returns the dmtcp configuration to be used for the container process.
func (e *EngineConfig) GetDMTCPConfig() DMTCPConfig {
	return e.JSON.DMTCPConfig
}

// SetXdgRuntimeDir sets a XDG_RUNTIME_DIR value for rootless operations
func (e *EngineConfig) SetXdgRuntimeDir(path string) {
	e.JSON.XdgRuntimeDir = path
}

// GetXdgRuntimeDir gets the XDG_RUNTIME_DIR value for rootless operations
func (e *EngineConfig) GetXdgRuntimeDir() string {
	return e.JSON.XdgRuntimeDir
}

// SetDbusSessionBusAddress sets a DBUS_SESSION_BUS_ADDRESS value for rootless operations
func (e *EngineConfig) SetDbusSessionBusAddress(address string) {
	e.JSON.DbusSessionBusAddress = address
}

// GetDbusSessionBusAddress gets the DBUS_SESSION_BUS_ADDRESS value for rootless operations
func (e *EngineConfig) GetDbusSessionBusAddress() string {
	return e.JSON.DbusSessionBusAddress
}

// SetNoEval sets whether to avoid a shell eval on APPTAINERENV_ and in
// runscripts generated from OCI containers CMD/ENTRYPOINT.
func (e *EngineConfig) SetNoEval(noEval bool) {
	e.JSON.NoEval = noEval
}

// GetNoEval sets whether to avoid a shell eval on APPTAINERENV_ and in
// runscripts generated from OCI containers CMD/ENTRYPOINT.
func (e *EngineConfig) GetNoEval() bool {
	return e.JSON.NoEval
}

// SetUnderlay sets whether to use underlay instead of overlay
func (e *EngineConfig) SetUnderlay(underlay bool) {
	e.JSON.Underlay = underlay
}

// GetUnderlay gets the value of whether to use underlay instead of overlay
func (e *EngineConfig) GetUnderlay() bool {
	return e.JSON.Underlay
}

// SetWritableOverlay sets whether the overlay is writable or not
func (e *EngineConfig) SetWritableOverlay(writableOverlay bool) {
	e.JSON.WritableOverlay = writableOverlay
}

// GetWritableOverlay gets the value of whether the overlay is writable or not
func (e *EngineConfig) GetWritableOverlay() bool {
	return e.JSON.WritableOverlay
}

// SetOverlayImplied sets whether the overlay was implied
// as opposed to explicitly requested
func (e *EngineConfig) SetOverlayImplied(overlayImplied bool) {
	e.JSON.OverlayImplied = overlayImplied
}

// GetOverlayImplied gets the value of whether the overlay was implied
func (e *EngineConfig) GetOverlayImplied() bool {
	return e.JSON.OverlayImplied
}

// SetShareNSMode sets whether container should run in shared namespace mode
func (e *EngineConfig) SetShareNSMode(mode bool) {
	e.JSON.ShareNSMode = mode
}

// GetShareNSMode gets the value of previous SetShareNSMode
func (e *EngineConfig) GetShareNSMode() bool {
	return e.JSON.ShareNSMode
}

// SetShareNSFd sets the locked fd
func (e *EngineConfig) SetShareNSFd(fd int) {
	e.JSON.ShareNSFd = fd
}

// GetShareNSFd gets the locked fd
func (e *EngineConfig) GetShareNSFd() int {
	return e.JSON.ShareNSFd
}

// SetRunscriptTimout sets the runscript timeout
func (e *EngineConfig) SetRunscriptTimout(timeout string) {
	e.JSON.RunscriptTimeout = timeout
}

// GetRunscriptTimeout gets the set runscript timeout
func (e *EngineConfig) GetRunscriptTimeout() string {
	return e.JSON.RunscriptTimeout
}
