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
// #nosec G101
const (
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteUse   string = `remote [remote options...]`
	RemoteShort string = `Manage apptainer remote endpoints`
	RemoteLong  string = `
  The 'remote' command allows you to manage Apptainer remote endpoints through
  its subcommands.

  A 'remote endpoint' is a group of services that is compatible with the
  container library API.  The remote endpoint is a single address,
  e.g. 'cloud.example.com' through which library and/or keystore services
  will be automatically discovered.

  To configure a remote endpoint you must 'remote add' it. You can 'remote login' if
  you will be performing actions needing authentication. Switch between
  configured remote endpoints with the 'remote use' command. The active remote
  endpoint will be used for key operations, and 'library://' pull
  and push. You can also 'remote logout' from and 'remote remove' an endpoint that
  is no longer required.

  The remote configuration is stored in $HOME/.apptainer/remotes.yaml by default.`
	RemoteExample string = `
  All group commands have their own help output:

    $ apptainer help remote list
    $ apptainer remote list`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote get-login-password
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteGetLoginPasswordUse     string = `get-login-password`
	RemoteGetLoginPasswordShort   string = `Retrieves the cli secret for the current logged in user`
	RemoteGetLoginPasswordLong    string = `The 'remote get-login-password' command allows you to retrieve the cli secret for the current user.`
	RemoteGetLoginPasswordExample string = `$ apptainer remote get-login-password | docker login -u user --password-stdin`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote add command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteAddUse   string = `add [add options...] <remote_name> <remote_URI>`
	RemoteAddShort string = `Add a new apptainer remote endpoint`
	RemoteAddLong  string = `
  The 'remote add' command allows you to add a new remote endpoint to be
  used for apptainer remote services. Authentication with a newly created
  endpoint will occur automatically.`
	RemoteAddExample string = `
  $ apptainer remote add ExampleCloud cloud.example.com`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote remove command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteRemoveUse   string = `remove [remove options...] <remote_name>`
	RemoteRemoveShort string = `Remove an existing apptainer remote endpoint`
	RemoteRemoveLong  string = `
  The 'remote remove' command allows you to remove an existing remote endpoint 
  from the list of potential endpoints to use.`
	RemoteRemoveExample string = `
  $ apptainer remote remove ExampleCloud`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote use command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteUseUse   string = `use [use options...] <remote_name>`
	RemoteUseShort string = `Set an Apptainer remote endpoint to be actively used`
	RemoteUseLong  string = `
  The 'remote use' command sets the remote to be used by default by any command
  that interacts with Apptainer services.`
	RemoteUseExample string = `
  $ apptainer remote use ExampleCloud`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote list command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteListUse   string = `list`
	RemoteListShort string = `List all apptainer remote endpoints that are configured`
	RemoteListLong  string = `
  The 'remote list' command lists all remote endpoints configured for use.

  The current remote is indicated by 'YES' in the 'ACTIVE' column and can be changed
  with the 'remote use' command.`
	RemoteListExample string = `
  $ apptainer remote list`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote login command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteLoginUse   string = `login [login options...] <remote_name>`
	RemoteLoginShort string = `Login to an apptainer remote endpoint`
	RemoteLoginLong  string = `
  The 'remote login' command allows you to set credentials for a specific
  endpoint.

  If no endpoint or registry is specified, the command will login to the currently
  active remote endpoint.`
	RemoteLoginExample string = `
  To log in to an endpoint:
  $ apptainer remote login SylabsCloud`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote logout command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteLogoutUse   string = `logout <remote_name>`
	RemoteLogoutShort string = `Log out from an apptainer remote endpoint`
	RemoteLogoutLong  string = `
  The 'remote logout' command allows you to log out from an apptainer specific
  endpoint. If no endpoint or service is specified, it will logout from the
  currently active remote endpoint.
  `
	RemoteLogoutExample string = `
  To log out from an endpoint
  $ apptainer remote logout SylabsCloud`
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// remote status command
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	RemoteStatusUse   string = `status [remote_name]`
	RemoteStatusShort string = `Check the status of the apptainer services at an endpoint, and your authentication token`
	RemoteStatusLong  string = `
  The 'remote status' command checks the status of the specified remote endpoint
  and reports the availability of services and their versions, and reports the
  user's logged-in status (or lack thereof) on that endpoint. If no endpoint is
  specified, it will check the status of the default remote. If
  you have logged in with an authentication token the validity of that token
  will be checked.`
	RemoteStatusExample string = `
  $ apptainer remote status`
)
