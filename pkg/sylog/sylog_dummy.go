// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build !sylog

package sylog

import (
	"io"
	"os"
)

var (
	noColorLevel messageLevel = 90
	loggerLevel               = InfoLevel
)

func getLoggerLevel() messageLevel {
	if loggerLevel <= -noColorLevel {
		return loggerLevel + noColorLevel
	} else if loggerLevel >= noColorLevel {
		return loggerLevel - noColorLevel
	}
	return loggerLevel
}

// Fatalf is a dummy function exiting with code 255. This
// function must not be used in public packages.
func Fatalf(format string, a ...interface{}) {
	os.Exit(255)
}

// Errorf is a dummy function doing nothing.
func Errorf(format string, a ...interface{}) {}

// Warningf is a dummy function doing nothing.
func Warningf(format string, a ...interface{}) {}

// Infof is a dummy function doing nothing.
func Infof(format string, a ...interface{}) {}

// Verbosef is a dummy function doing nothing.
func Verbosef(format string, a ...interface{}) {}

// Debugf is a dummy function doing nothing
func Debugf(format string, a ...interface{}) {}

// SetLevel is a dummy function doing nothing.
func SetLevel(l int, color bool) {
	// Here we do not check term.IsTerminal to explicitly control the color
	loggerLevel = messageLevel(l)
	if !color {
		if loggerLevel >= InfoLevel {
			loggerLevel = loggerLevel + noColorLevel
		} else if loggerLevel <= LogLevel {
			loggerLevel = loggerLevel - noColorLevel
		}
	}
}

// DisableColor for the logger
func DisableColor() {}

// GetLevel is a dummy function returning lowest message level.
func GetLevel() int {
	return int(getLoggerLevel())
}

// GetEnvVar is a dummy function returning environment variable
// with lowest message level.
func GetEnvVar() string {
	return "APPTAINER_MESSAGELEVEL=-1"
}

// Writer is a dummy function returning io.Discard writer.
func Writer() io.Writer {
	return io.Discard
}

// DebugLogger is an implementation of the go-log/log Logger interface that will
// output log messages via sylog.debug when required by external packages
type DebugLogger struct{}

// Log is a dummy function doing nothing.
func (t DebugLogger) Log(v ...interface{}) {}

// Logf is a dummy function doing nothing.
func (t DebugLogger) Logf(format string, v ...interface{}) {}
