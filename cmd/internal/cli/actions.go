// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/cache"
	"github.com/apptainer/apptainer/internal/pkg/client/ipfs"
	"github.com/apptainer/apptainer/internal/pkg/client/library"
	"github.com/apptainer/apptainer/internal/pkg/client/net"
	"github.com/apptainer/apptainer/internal/pkg/client/oci"
	"github.com/apptainer/apptainer/internal/pkg/client/oras"
	"github.com/apptainer/apptainer/internal/pkg/client/shub"
	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/internal/pkg/ociimage"
	"github.com/apptainer/apptainer/internal/pkg/runtime/launch"
	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/internal/pkg/util/uri"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/fs/lock"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const (
	defaultPath           = "/bin:/usr/bin:/sbin:/usr/sbin:/usr/local/bin:/usr/local/sbin"
	shareNSInstancePrefix = "sharens_instance"
)

func getCacheHandle(cfg cache.Config) *cache.Handle {
	envKey := env.TrimApptainerKey(cache.DirEnv)
	h, err := cache.New(cache.Config{
		ParentDir: env.GetenvLegacy(envKey, envKey),
		Disable:   cfg.Disable,
	})
	if err != nil {
		sylog.Fatalf("Failed to create an image cache handle: %s", err)
	}

	return h
}

// actionPreRun will run replaceURIWithImage and will also do the proper path unsetting
func actionPreRun(cmd *cobra.Command, args []string) {
	// For compatibility - we still set USER_PATH so it will be visible in the
	// container, and can be used there if needed. USER_PATH is not used by
	// apptainer itself in 1.0.0+
	userPath := strings.Join([]string{os.Getenv("PATH"), defaultPath}, ":")
	os.Setenv("USER_PATH", userPath)

	os.Setenv("IMAGE_ARG", args[0])

	replaceURIWithImage(cmd.Context(), cmd, args)

	// --compat infers other options that give increased OCI / Docker compatibility
	// Excludes uts/user/net namespaces as these are restrictive for many Apptainer
	// installs.
	if isCompat {
		isContainAll = true
		isWritableTmpfs = true
		noInit = true
		noUmask = true
		noEval = true
	}
}

func handleOCI(ctx context.Context, imgCache *cache.Handle, cmd *cobra.Command, pullFrom string) (string, error) {
	ociAuth, err := makeOCICredentials(cmd)
	if err != nil {
		sylog.Fatalf("While creating Docker credentials: %v", err)
	}

	pullOpts := oci.PullOptions{
		TmpDir:      tmpDir,
		OciAuth:     ociAuth,
		DockerHost:  dockerHost,
		NoHTTPS:     noHTTPS,
		ReqAuthFile: reqAuthFile,
		Platform:    getOCIPlatform(),
	}

	return oci.Pull(ctx, imgCache, pullFrom, pullOpts)
}

func handleOras(ctx context.Context, imgCache *cache.Handle, cmd *cobra.Command, pullFrom string) (string, error) {
	ociAuth, err := makeOCICredentials(cmd)
	if err != nil {
		return "", fmt.Errorf("while creating docker credentials: %v", err)
	}
	return oras.Pull(ctx, imgCache, pullFrom, tmpDir, ociAuth, noHTTPS, reqAuthFile)
}

func handleIpfs(ctx context.Context, imgCache *cache.Handle, pullFrom string) (string, error) {
	return ipfs.Pull(ctx, imgCache, pullFrom, tmpDir)
}

func handleLibrary(ctx context.Context, imgCache *cache.Handle, pullFrom string) (string, error) {
	r, err := library.NormalizeLibraryRef(pullFrom)
	if err != nil {
		return "", err
	}

	// Default "" = use current remote endpoint
	var libraryURI string
	if r.Host != "" {
		if noHTTPS {
			libraryURI = "http://" + r.Host
		} else {
			libraryURI = "https://" + r.Host
		}
	}

	c, err := getLibraryClientConfig(libraryURI)
	if err != nil {
		return "", err
	}
	pullOpts := library.PullOptions{
		LibraryConfig: c,
	}
	return library.Pull(ctx, imgCache, r, runtime.GOARCH, tmpDir, pullOpts)
}

func handleShub(ctx context.Context, imgCache *cache.Handle, pullFrom string) (string, error) {
	return shub.Pull(ctx, imgCache, pullFrom, tmpDir, noHTTPS)
}

func handleNet(ctx context.Context, imgCache *cache.Handle, pullFrom string) (string, error) {
	return net.Pull(ctx, imgCache, pullFrom, tmpDir)
}

func replaceURIWithImage(ctx context.Context, cmd *cobra.Command, args []string) {
	// If args[0] is not transport:ref (ex. instance://...) formatted return, not a URI
	t, _ := uri.Split(args[0])
	if t == "instance" || t == "" {
		return
	}

	var image string
	var err error

	// Create a cache handle only when we know we are using a URI
	imgCache := getCacheHandle(cache.Config{Disable: disableCache})
	if imgCache == nil {
		sylog.Fatalf("failed to create a new image cache handle")
	}

	switch t {
	case uri.Library:
		image, err = handleLibrary(ctx, imgCache, args[0])
	case uri.Oras:
		image, err = handleOras(ctx, imgCache, cmd, args[0])
	case uri.IPFS:
		image, err = handleIpfs(ctx, imgCache, args[0])
	case uri.Shub:
		image, err = handleShub(ctx, imgCache, args[0])
	case ociimage.SupportedTransport(t):
		image, err = handleOCI(ctx, imgCache, cmd, args[0])
	case uri.HTTP:
		image, err = handleNet(ctx, imgCache, args[0])
	case uri.HTTPS:
		image, err = handleNet(ctx, imgCache, args[0])
	default:
		sylog.Fatalf("Unsupported transport type: %s", t)
	}

	if err != nil {
		sylog.Fatalf("Unable to handle %s uri: %v", args[0], err)
	}

	args[0] = image
}

// ExecCmd represents the exec command
var ExecCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	TraverseChildren:      true,
	Args:                  cobra.MinimumNArgs(2),
	PreRun:                actionPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		a := append([]string{"/.singularity.d/actions/exec"}, args[1:]...)
		if shareNS {
			if err := shareNSLaunch(cmd, args[0], a); err != nil {
				sylog.Fatalf("%s", err)
			}
		} else {
			if err := launchContainer(cmd, args[0], a, "", -1); err != nil {
				sylog.Fatalf("%s", err)
			}
		}
	},

	Use:     docs.ExecUse,
	Short:   docs.ExecShort,
	Long:    docs.ExecLong,
	Example: docs.ExecExamples,
}

// ShellCmd represents the shell command
var ShellCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	TraverseChildren:      true,
	Args:                  cobra.MinimumNArgs(1),
	PreRun:                actionPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 {
			sylog.Warningf("Parameters to shell command are ignored")
		}

		a := []string{"/.singularity.d/actions/shell"}
		if shareNS {
			if err := shareNSLaunch(cmd, args[0], a); err != nil {
				sylog.Fatalf("%s", err)
			}
		} else {
			if err := launchContainer(cmd, args[0], a, "", -1); err != nil {
				sylog.Fatalf("%s", err)
			}
		}
	},

	Use:     docs.ShellUse,
	Short:   docs.ShellShort,
	Long:    docs.ShellLong,
	Example: docs.ShellExamples,
}

// RunCmd represents the run command
var RunCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	TraverseChildren:      true,
	Args:                  cobra.MinimumNArgs(1),
	PreRun:                actionPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		a := append([]string{"/.singularity.d/actions/run"}, args[1:]...)
		if shareNS {
			if err := shareNSLaunch(cmd, args[0], a); err != nil {
				sylog.Fatalf("%s", err)
			}
		} else {
			if err := launchContainer(cmd, args[0], a, "", -1); err != nil {
				sylog.Fatalf("%s", err)
			}
		}
	},

	Use:     docs.RunUse,
	Short:   docs.RunShort,
	Long:    docs.RunLong,
	Example: docs.RunExamples,
}

// TestCmd represents the test command
var TestCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	TraverseChildren:      true,
	Args:                  cobra.MinimumNArgs(1),
	PreRun:                actionPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		a := append([]string{"/.singularity.d/actions/test"}, args[1:]...)
		if shareNS {
			if err := shareNSLaunch(cmd, args[0], a); err != nil {
				sylog.Fatalf("%s", err)
			}
		} else {
			if err := launchContainer(cmd, args[0], a, "", -1); err != nil {
				sylog.Fatalf("%s", err)
			}
		}
	},

	Use:     docs.RunTestUse,
	Short:   docs.RunTestShort,
	Long:    docs.RunTestLong,
	Example: docs.RunTestExample,
}

func launchContainer(cmd *cobra.Command, image string, args []string, instanceName string, fd int) error {
	ns := launch.Namespaces{
		User:  userNamespace,
		UTS:   utsNamespace,
		PID:   pidNamespace,
		IPC:   ipcNamespace,
		Net:   netNamespace,
		NoPID: noPidNamespace,
	}

	cgJSON, err := getCgroupsJSON()
	if err != nil {
		return err
	}
	if cgJSON != "" && strings.HasPrefix(image, "instance://") {
		cgJSON = ""
		sylog.Warningf("Resource limits & cgroups configuration are only applied to instances at instance start.")
	}

	ki, err := getEncryptionMaterial(cmd)
	if err != nil {
		return err
	}

	opts := []launch.Option{
		launch.OptWritable(isWritable),
		launch.OptWritableTmpfs(isWritableTmpfs),
		launch.OptOverlayPaths(overlayPath),
		launch.OptScratchDirs(scratchPath),
		launch.OptWorkDir(workdirPath),
		launch.OptHome(
			homePath,
			cmd.Flag(actionHomeFlag.Name).Changed,
			noHome,
		),
		launch.OptMounts(bindPaths, mounts, fuseMount),
		launch.OptNoMount(noMount),
		launch.OptNvidia(nvidia, nvCCLI),
		launch.OptNoNvidia(noNvidia),
		launch.OptRocm(rocm),
		launch.OptNoRocm(noRocm),
		launch.OptContainLibs(containLibsPath),
		launch.OptEnv(apptainerEnv,
			apptainerNoEnv,
			apptainerEnvFiles,
			isCleanEnv),
		launch.OptNoEval(noEval),
		launch.OptNamespaces(ns),
		launch.OptNetnsPath(netnsPath),
		launch.OptNetwork(network, networkArgs),
		launch.OptHostname(hostname),
		launch.OptDNS(dns),
		launch.OptCaps(addCaps, dropCaps),
		launch.OptAllowSUID(allowSUID),
		launch.OptKeepPrivs(keepPrivs),
		launch.OptNoPrivs(noPrivs),
		launch.OptSecurity(security),
		launch.OptNoUmask(noUmask),
		launch.OptCgroupsJSON(cgJSON),
		launch.OptConfigFile(configurationFile),
		launch.OptShellPath(shellPath),
		launch.OptCwdPath(cwdPath),
		launch.OptFakeroot(isFakeroot),
		launch.OptBoot(isBoot),
		launch.OptNoInit(noInit),
		launch.OptContain(isContained),
		launch.OptContainAll(isContainAll),
		launch.OptAppName(appName),
		launch.OptKeyInfo(ki),
		launch.OptCacheDisabled(disableCache),
		launch.OptDevice(device),
		launch.OptCdiDirs(cdiDirs),
		launch.OptDMTCPLaunch(dmtcpLaunch),
		launch.OptDMTCPRestart(dmtcpRestart),
		launch.OptUnsquash(unsquash),
		launch.OptIgnoreSubuid(ignoreSubuid),
		launch.OptIgnoreFakerootCmd(ignoreFakerootCmd),
		launch.OptIgnoreUserns(ignoreUserns),
		launch.OptUseBuildConfig(useBuildConfig),
		launch.OptTmpDir(tmpDir),
		launch.OptUnderlay(underlay),
		launch.OptShareNSMode(shareNS),
		launch.OptShareNSFd(fd),
		launch.OptRunscriptTimeout(runscriptTimeout),
		launch.OptIntelHpu(intelHpu),
	}

	l, err := launch.NewLauncher(opts...)
	if err != nil {
		return fmt.Errorf("while configuring container: %s", err)
	}

	return l.Exec(cmd.Context(), image, args, instanceName)
}

func shareNSLaunch(cmd *cobra.Command, image string, args []string) error {
	ppid := os.Getppid()
	lockFile := fmt.Sprintf("%s/%s_%d", "/dev/shm", shareNSInstancePrefix, ppid)
	instanceName := fmt.Sprintf("%s_%d", shareNSInstancePrefix, ppid)

	var fd int
	var err error
	existingInstanceLock := false
	fd, err = unix.Open(lockFile, unix.O_CREAT|unix.O_RDWR|unix.O_EXCL, 0o700)
	if err != nil {
		existingInstanceLock = err == syscall.EEXIST
		if !existingInstanceLock {
			return err
		}

		fd, err = unix.Open(lockFile, unix.O_RDWR, 0o700)
		if err != nil {
			return err
		}
	}

	br := lock.NewByteRange(fd, 0, 0)
	err = br.Lock()
	if err != nil && err != lock.ErrByteRangeAcquired {
		return err
	}

	firstProcess := err == nil

	// check existingInstanceLock, if true and we can acquire a lock
	// it means the instance has been created and we are not the first
	// process
	if firstProcess && existingInstanceLock {
		// read the content of the lock file (count up to 1 byte)
		buf := make([]byte, 1)
		n, err := unix.Pread(fd, buf, io.SeekStart)
		if err != nil {
			return err
		}
		// if there is no content, meaning that previous startup fails
		// because the successful first process will always write 1 byte into this lock file
		firstProcess = n == 0
	}

	if firstProcess {
		if err := launchContainer(cmd, image, args, instanceName, fd); err != nil {
			return err
		}
	} else {
		// other process, this will block the process
		if err := br.RLockw(); err != nil {
			return err
		} else if _, err := instance.Get(instanceName, instance.AppSubDir); err != nil {
			return fmt.Errorf("first process with --sharens has already exited, could not execute process (pid %d)", os.Getpid())
		}
		if err := launchContainer(cmd, fmt.Sprintf("instance://%s", instanceName), args, "", -1); err != nil {
			return err
		}
	}

	return nil
}
