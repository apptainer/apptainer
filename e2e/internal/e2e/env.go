// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

// TestEnv stores all the information under the control of e2e test developers,
// from specifying which Apptainer binary to use to controlling how Apptainer
// environment variables will be set.
type TestEnv struct {
	CmdPath               string // Path to the Apptainer binary to use for the execution of an Apptainer command
	ImagePath             string // Path to the image that has to be used for the execution of an Apptainer command
	SingularityImagePath  string // Path to a Singularity image for legacy tests
	DebianImagePath       string // Path to an image containing a Debian distribution with libc compatible to the host libc
	OrasTestImage         string // URI to SIF image pushed into local registry with ORAS
	TestDir               string // Path to the directory from which an Apptainer command needs to be executed
	TestRegistry          string // Host:Port of local registry
	TestRegistryImage     string // URI to OCI image pushed into local registry
	TestRegistryPrivPath  string // Host:Port of local registry + path to private location
	TestRegistryPrivURI   string // Transport (docker://) + Host:Port of local registry + path to private location
	TestRegistryPrivImage string // URI to OCI image pushed into private location in local registry
	HomeDir               string // HomeDir sets the home directory that will be used for the execution of a command
	KeyringDir            string // KeyringDir sets the directory where the keyring will be created for the execution of a command (instead of using APPTAINER_KEYSDIR which should be avoided when running e2e tests)
	PrivCacheDir          string // PrivCacheDir sets the location of the image cache to be used by the Apptainer command to be executed as root (instead of using APPTAINER_CACHE_DIR which should be avoided when running e2e tests)
	UnprivCacheDir        string // UnprivCacheDir sets the location of the image cache to be used by the Apptainer command to be executed as the unpriv user (instead of using APPTAINER_CACHE_DIR which should be avoided when running e2e tests)
	RunDisabled           bool
	DisableCache          bool   // DisableCache can be set to disable the cache during the execution of a e2e command
	InsecureRegistry      string // Insecure registry replaced with nip.io
}
