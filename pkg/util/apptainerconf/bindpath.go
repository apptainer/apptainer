// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainerconf

import (
	"errors"
	"fmt"
	"strings"
)

const bindPathDirective = "bind path"

// ErrUnsupportedBindOption is returned by ParseBindPath, along with the
// parsed entry, when a bind path has mount options it does not support.
var ErrUnsupportedBindOption = errors.New("unsupported mount option")

// BindPathEntry is a parsed "bind path" directive value of the form
// <src>[:<dst>[:<options>]].
type BindPathEntry struct {
	Source      string
	Destination string
	// Dev prevents nodev from being added to the bind mount.
	Dev bool
	// Suid prevents nosuid from being added to the bind mount.
	Suid bool
}

// SplitDirectiveValue splits a raw directive value on commas. For the
// "bind path" directive, empty tokens are dropped and a token that is not an
// absolute path is a mount option that stays attached to the preceding
// entry, so that "/a::dev,suid,/b" yields "/a::dev,suid" and "/b".
func SplitDirectiveValue(directive, value string) []string {
	var values []string
	for _, v := range strings.Split(value, ",") {
		v = strings.TrimSpace(v)
		if directive == bindPathDirective {
			if v == "" {
				continue
			}
			if len(values) > 0 && !strings.HasPrefix(v, "/") {
				values[len(values)-1] += "," + v
				continue
			}
		}
		values = append(values, v)
	}
	return values
}

// ParseBindPath parses a "bind path" directive value of the form
// <src>[:<dst>[:<options>]]. An empty destination means the same as the
// source. Supported options are "dev" and "suid"; if there are others, the
// parsed entry is returned with an error wrapping ErrUnsupportedBindOption.
func ParseBindPath(entry string) (BindPathEntry, error) {
	fields := strings.SplitN(entry, ":", 3)

	bp := BindPathEntry{Source: fields[0]}
	if len(fields) > 1 {
		bp.Destination = fields[1]
	}
	if bp.Destination == "" {
		bp.Destination = bp.Source
	}

	if bp.Source == "" {
		return bp, fmt.Errorf("bind path %q: empty source", entry)
	}
	if strings.Contains(bp.Source, ",") || strings.Contains(bp.Destination, ",") {
		return bp, fmt.Errorf("bind path %q: mount options must follow the destination, as <src>:<dst>:<options>", entry)
	}

	if len(fields) < 3 {
		return bp, nil
	}
	var unsupported []string
	for _, opt := range strings.Split(fields[2], ",") {
		switch opt = strings.TrimSpace(opt); opt {
		case "":
		case "dev":
			bp.Dev = true
		case "suid":
			bp.Suid = true
		default:
			unsupported = append(unsupported, fmt.Sprintf("%q", opt))
		}
	}
	if len(unsupported) > 0 {
		return bp, fmt.Errorf("bind path %q: %w %s (supported: dev, suid)",
			entry, ErrUnsupportedBindOption, strings.Join(unsupported, ", "))
	}

	return bp, nil
}
