// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

// TestEnv stores all the information under the control of e2e test developers,
// from specifying which Apptainer binary to use to controlling how Apptainer
// environment variables will be set.
type TestEnv struct {
	CmdPath       string // Path to the Apptainer binary to use for the execution of a Apptainer command
	ImagePath     string // Path to the image that has to be used for the execution of a Apptainer command
	OrasTestImage string
	TestDir       string // Path to the directory from which a Apptainer command needs to be executed
	TestRegistry  string
	KeyringDir    string // KeyringDir sets the directory where the keyring will be created for the execution of a command (instead of using APPTAINER_SYPGPDIR which should be avoided when running e2e tests)
	ImgCacheDir   string // ImgCacheDir sets the location of the image cache to be used by the Apptainer command to be executed (instead of using APPTAINER_CACHE_DIR which should be avoided when running e2e tests)
	RunDisabled   bool
	DisableCache  bool // DisableCache can be set to disable the cache during the execution of a e2e command
}
