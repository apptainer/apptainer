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
	// keyserver command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	KeyserverUse   string = `keyserver [subcommand options...]`
	KeyserverShort string = `Manage apptainer keyservers`
	KeyserverLong  string = `
  The 'keyserver' command allows you to manage standalone keyservers that will 
  be used for retrieving cryptographic keys.`
	KeyserverExample string = `
  All group commands have their own help output:

    $ apptainer help keyserver add
    $ apptainer keyserver add`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// keyserver add command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	KeyserverAddUse   string = `add [options] [remoteName] <keyserver_url>`
	KeyserverAddShort string = `Add a keyserver (root user only)`
	KeyserverAddLong  string = `
  The 'keyserver add' command lets the user specify an additional keyserver.
  The --order specifies the order of the new keyserver relative to the 
  keyservers that have already been specified. Therefore, when specifying
  '--order 1', the new keyserver will become the primary one. If no endpoint is
  specified, the new keyserver will be associated with the default remote
  endpoint.`
	KeyserverAddExample string = `
  $ apptainer keyserver add https://keys.example.com

  To add a keyserver to be used as the primary keyserver for the current
  endpoint:
  $ apptainer keyserver add --order 1 https://keys.example.com`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// keyserver remove command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	KeyserverRemoveUse   string = `remove [remoteName] <keyserver_url>`
	KeyserverRemoveShort string = `Remove a keyserver (root user only)`
	KeyserverRemoveLong  string = `
  The 'keyserver remove' command lets the user remove a previously specified
  keyserver from a specific endpoint. If no endpoint is specified, the default
  remote endpoint will be assumed.`
	KeyserverRemoveExample string = `
  $ apptainer keyserver remove https://keys.example.com`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// keyserver login command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	KeyserverLoginUse   string = `login [login options...] <keyserver>`
	KeyserverLoginShort string = `Login to a keyserver`
	KeyserverLoginLong  string = `
  The 'keyserver login' command allows you to login to a specific keyserver.`
	KeyserverLoginExample string = `
  To login in to a keyserver:
  $ apptainer keyserver login --username foo https://mykeyserver.example.com`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// keyserver logout command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	KeyserverLogoutUse   string = `logout <keyserver>`
	KeyserverLogoutShort string = `Logout from a keyserver`
	KeyserverLogoutLong  string = `
  The 'keyserver logout' command allows you to log out from a keyserver.`
	KeyserverLogoutExample string = `
  To log out from a keyserver:
  $ apptainer keyserver logout https://mykeyserver.example.com`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// keyserver list command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	KeyserverListUse   string = `list [remoteName]`
	KeyserverListShort string = `List all keyservers that are configured`
	KeyserverListLong  string = `
  The 'keyserver list' command lists all keyservers configured for use with a
  given remote endpoint. If no endpoint is specified, the default
  remote endpoint will be assumed.`
	KeyserverListExample string = `
  $ apptainer keyserver list`
)
