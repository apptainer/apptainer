// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build seccomp

package seccomp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/opencontainers/runtime-spec/specs-go"
	cseccomp "github.com/seccomp/containers-golang"
	lseccomp "github.com/seccomp/libseccomp-golang"
)

var scmpArchMap = map[specs.Arch]lseccomp.ScmpArch{
	"":                    lseccomp.ArchNative,
	specs.ArchX86:         lseccomp.ArchX86,
	specs.ArchX86_64:      lseccomp.ArchAMD64,
	specs.ArchX32:         lseccomp.ArchX32,
	specs.ArchARM:         lseccomp.ArchARM,
	specs.ArchAARCH64:     lseccomp.ArchARM64,
	specs.ArchMIPS:        lseccomp.ArchMIPS,
	specs.ArchMIPS64:      lseccomp.ArchMIPS64,
	specs.ArchMIPS64N32:   lseccomp.ArchMIPS64N32,
	specs.ArchMIPSEL:      lseccomp.ArchMIPSEL,
	specs.ArchMIPSEL64:    lseccomp.ArchMIPSEL64,
	specs.ArchMIPSEL64N32: lseccomp.ArchMIPSEL64N32,
	specs.ArchPPC:         lseccomp.ArchPPC,
	specs.ArchPPC64:       lseccomp.ArchPPC64,
	specs.ArchPPC64LE:     lseccomp.ArchPPC64LE,
	specs.ArchS390:        lseccomp.ArchS390,
	specs.ArchS390X:       lseccomp.ArchS390X,
}

var scmpActionMap = map[specs.LinuxSeccompAction]lseccomp.ScmpAction{
	specs.ActKill:  lseccomp.ActKillThread,
	specs.ActTrap:  lseccomp.ActTrap,
	specs.ActErrno: lseccomp.ActErrno,
	specs.ActTrace: lseccomp.ActTrace,
	specs.ActAllow: lseccomp.ActAllow,
}

var scmpCompareOpMap = map[specs.LinuxSeccompOperator]lseccomp.ScmpCompareOp{
	specs.OpNotEqual:     lseccomp.CompareNotEqual,
	specs.OpLessThan:     lseccomp.CompareLess,
	specs.OpLessEqual:    lseccomp.CompareLessOrEqual,
	specs.OpEqualTo:      lseccomp.CompareEqual,
	specs.OpGreaterEqual: lseccomp.CompareGreaterEqual,
	specs.OpGreaterThan:  lseccomp.CompareGreater,
	specs.OpMaskedEqual:  lseccomp.CompareMaskedEqual,
}

func prctl(option uintptr, arg2 uintptr, arg3 uintptr, arg4 uintptr, arg5 uintptr) syscall.Errno {
	_, _, err := syscall.Syscall6(syscall.SYS_PRCTL, option, arg2, arg3, arg4, arg5, 0)
	return err
}

func hasConditionSupport() bool {
	major, minor, micro := lseccomp.GetLibraryVersion()
	return (major > 2) || (major == 2 && minor >= 2) || (major == 2 && minor == 2 && micro >= 1)
}

// Enabled returns whether seccomp is enabled.
func Enabled() bool {
	return true
}

// LoadSeccompConfig loads seccomp configuration filter for the current process.
func LoadSeccompConfig(config *specs.LinuxSeccomp, noNewPrivs bool, errNo int16) error {
	if err := prctl(syscall.PR_GET_SECCOMP, 0, 0, 0, 0); err == syscall.EINVAL {
		return fmt.Errorf("can't load seccomp filter: not supported by kernel")
	}

	if err := prctl(syscall.PR_SET_SECCOMP, 2, 0, 0, 0); err == syscall.EINVAL {
		return fmt.Errorf("can't load seccomp filter: SECCOMP_MODE_FILTER not supported")
	}

	if config == nil {
		return fmt.Errorf("empty config passed")
	}

	if len(config.DefaultAction) == 0 {
		return fmt.Errorf("a defaultAction must be provided")
	}

	supportCondition := hasConditionSupport()
	if !supportCondition {
		sylog.Warningf("seccomp rule conditions are not supported with libseccomp under 2.2.1")
	}

	scmpDefaultAction, ok := scmpActionMap[config.DefaultAction]
	if !ok {
		return fmt.Errorf("invalid action '%s' specified", config.DefaultAction)
	}
	if scmpDefaultAction == lseccomp.ActErrno {
		scmpDefaultAction = scmpDefaultAction.SetReturnCode(errNo)
	}

	scmpNativeArch, err := lseccomp.GetNativeArch()
	if err != nil {
		return fmt.Errorf("failed to get seccomp native architecture: %s", err)
	}

	scmpArchitectures := []lseccomp.ScmpArch{scmpNativeArch}

	for _, arch := range config.Architectures {
		scmpArch, ok := scmpArchMap[arch]
		if !ok {
			return fmt.Errorf("invalid architecture '%s' specified", arch)
		} else if scmpArch != scmpNativeArch {
			scmpArchitectures = append(scmpArchitectures, scmpArch)
		}
	}

	var mergeFilter *lseccomp.ScmpFilter

	for _, scmpArch := range scmpArchitectures {
		filter, err := lseccomp.NewFilter(scmpDefaultAction)
		if err != nil {
			return fmt.Errorf("error creating new filter: %s", err)
		}

		if err := filter.SetNoNewPrivsBit(noNewPrivs); err != nil {
			return fmt.Errorf("failed to set no new priv flag: %s", err)
		}

		if scmpArch != scmpNativeArch {
			if err := filter.AddArch(scmpArch); err != nil {
				return fmt.Errorf("error adding architecture: %s", err)
			}

			if err := filter.RemoveArch(scmpNativeArch); err != nil {
				return fmt.Errorf("error removing architecture: %s", err)
			}
		}

		ignoreArchFilter := false

		for _, syscall := range config.Syscalls {
			if len(syscall.Names) == 0 {
				return fmt.Errorf("no syscall specified for the rule")
			}

			scmpAction, ok := scmpActionMap[syscall.Action]
			if !ok {
				return fmt.Errorf("invalid action '%s' specified", syscall.Action)
			}
			if scmpAction == lseccomp.ActErrno {
				scmpAction = scmpAction.SetReturnCode(errNo)
			}

			for _, sysName := range syscall.Names {
				sysNr, err := lseccomp.GetSyscallFromNameByArch(sysName, scmpArch)
				if err != nil {
					continue
				}

				if len(syscall.Args) == 0 || !supportCondition {
					if err := filter.AddRule(sysNr, scmpAction); err != nil {
						if isUnrecognizedSyscall(err) {
							ignoreArchFilter = true
							break
						}
						return fmt.Errorf("failed adding seccomp rule for syscall %s: %s", sysName, err)
					}
				} else {
					conditions, err := addSyscallRuleConditions(syscall.Args)
					if err != nil {
						return err
					}
					if err := filter.AddRuleConditional(sysNr, scmpAction, conditions); err != nil {
						if isUnrecognizedSyscall(err) {
							ignoreArchFilter = true
							break
						}
						return fmt.Errorf("failed adding rule condition for syscall %s: %s", sysName, err)
					}
				}
			}
		}

		if !ignoreArchFilter {
			if mergeFilter == nil {
				mergeFilter = filter
			} else {
				if err := mergeFilter.Merge(filter); err != nil {
					return fmt.Errorf("failed to merge seccomp filter for architecture %s: %s", scmpArch, err)
				}
			}
		}
	}

	if mergeFilter == nil {
		return fmt.Errorf("seccomp filter not applied due to error")
	} else if err := mergeFilter.Load(); err != nil {
		return fmt.Errorf("failed loading seccomp filter: %s", err)
	}

	return nil
}

func isUnrecognizedSyscall(err error) bool {
	return strings.Contains(err.Error(), "unrecognized syscall")
}

func addSyscallRuleConditions(args []specs.LinuxSeccompArg) ([]lseccomp.ScmpCondition, error) {
	var maxIndex uint = 6
	conditions := make([]lseccomp.ScmpCondition, 0)

	for _, arg := range args {
		if arg.Index >= maxIndex {
			return conditions, fmt.Errorf("the maximum index of syscall arguments is %d: given %d", maxIndex, arg.Index)
		}
		operator, ok := scmpCompareOpMap[arg.Op]
		if !ok {
			return conditions, fmt.Errorf("invalid operator encountered %s", arg.Op)
		}
		cond, err := lseccomp.MakeCondition(arg.Index, operator, arg.Value, arg.ValueTwo)
		if err != nil {
			return conditions, fmt.Errorf("error making syscall rule condition: %s", err)
		}
		conditions = append(conditions, cond)
	}

	return conditions, nil
}

// LoadProfileFromFile loads seccomp rules from json file and fill in provided OCI configuration.
func LoadProfileFromFile(profile string, generator *generate.Generator) error {
	file, err := os.Open(profile)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if generator.Config.Linux == nil {
		generator.Config.Linux = &specs.Linux{}
	}
	if generator.Config.Process == nil {
		generator.Config.Process = &specs.Process{}
	}
	if generator.Config.Process.Capabilities == nil {
		generator.Config.Process.Capabilities = &specs.LinuxCapabilities{}
	}

	seccompConfig, err := cseccomp.LoadProfileFromBytes(data, generator.Config)
	if err != nil {
		return err
	}
	generator.Config.Linux.Seccomp = seccompConfig

	return nil
}
