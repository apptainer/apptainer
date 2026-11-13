// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

//go:build !getent_fallback || !linux

package user

// Without the getent_fallback build tag, lookups use only the in-process
// implementation.

func getentUserFallback(_ string, _ bool, u *User, err error) (*User, error) {
	return u, err
}

func getentGroupFallback(_ string, _ bool, g *Group, err error) (*Group, error) {
	return g, err
}
