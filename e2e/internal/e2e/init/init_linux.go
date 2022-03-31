// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package init

/*
#define _GNU_SOURCE
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <sched.h>
#include <sys/mount.h>
#include <sys/types.h>
#include <sys/wait.h>

// This is the CGO init constructor called before executing any Go code
// in e2e/e2e_test.go.
__attribute__((constructor)) static void init(void) {
	uid_t uid = 0;
	gid_t gid = 0;

	if ( getuid() != 0 ) {
		fprintf(stderr, "tests must be executed as root user\n");
		fprintf(stderr, "%d %d", getuid(), getgid());
		exit(1);
	} else if ( getenv("E2E_NO_REAPER") == NULL ) {
		return;
	}

	if ( getenv("E2E_ORIG_GID") == NULL ) {
		fprintf(stderr, "E2E_ORIG_GID environment variable not set\n");
	}
	gid = atoi(getenv("E2E_ORIG_GID"));

	if ( getenv("E2E_ORIG_UID") == NULL ) {
		fprintf(stderr, "E2E_ORIG_UID environment variable not set\n");
	}
	uid = atoi(getenv("E2E_ORIG_UID"));

	if ( mount(NULL, "/", NULL, MS_PRIVATE|MS_REC, NULL) < 0 ) {
		fprintf(stderr, "failed to set private mount propagation: %s\n", strerror(errno));
		exit(1);
	}

	// set original user identity and retain privileges for
	// Privileged method
	if ( setresgid(gid, gid, 0) < 0 ) {
		fprintf(stderr, "setresgid failed: %s\n", strerror(errno));
		exit(1);
	}
	if ( setresuid(uid, uid, 0) < 0 ) {
		fprintf(stderr, "setresuid failed: %s\n", strerror(errno));
		exit(1);
	}
}
*/
import "C"
