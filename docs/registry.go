// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package docs

// Global content for help and man pages
const (

	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// registry command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RegistryUse   string = `registry [subcommand options...]`
	RegistryShort string = `Manage authentication to OCI/Docker registries`
	RegistryLong  string = `
  The 'registry' command allows you to manage authentication to standalone OCI/Docker
  registries, such as 'docker://'' or 'oras://'.`
	RegistryExample string = `
  All group commands have their own help output:

    $ apptainer help registry login
    $ apptainer registry login`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// registry login command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RegistryLoginUse   string = `login [login options...] <registry_uri>`
	RegistryLoginShort string = `Login to an OCI/Docker registry`
	RegistryLoginLong  string = `
  The 'registry login' command allows you to login to a specific OCI/Docker
  registry.`
	RegistryLoginExample string = `
  To login in to a docker/OCI registry:
  $ apptainer registry login --username foo docker://docker.io
  $ apptainer registry login --username foo oras://myregistry.example.com

  Note that many cloud OCI registries use token-based authentication. The token
  should be specified as the password for login. A username is still required.
  E.g. when using a standard Azure identity and token to login to an ACR 
  registry, the username '00000000-0000-0000-0000-000000000000' is required.
  Consult your provider's documentation for details concerning their specific
  login requirements.`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// registry logout command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RegistryLogoutUse   string = `logout <registry_uri>`
	RegistryLogoutShort string = `Logout from an OCI/Docker registry`
	RegistryLogoutLong  string = `
  The 'registry logout' command allows you to log out from an OCI/Docker
  registry.`
	RegistryLogoutExample string = `
  To log out from an OCI/Docker registry
  $ apptainer registry logout docker://docker.io`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// registry list command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RegistryListUse   string = `list`
	RegistryListShort string = `List all OCI credentials that are configured`
	RegistryListLong  string = `
  The 'registry list' command lists all credentials for OCI/Docker registries
  that are configured for use.`
	RegistryListExample string = `
  $ apptainer registry list`
)
