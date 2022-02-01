/*
 * Copyright (c) Contributors to the Apptainer project, established as
 *   Apptainer a Series of LF Projects LLC.
 *   For website terms of use, trademark policy, privacy policy and other
 *   project policies see https://lfprojects.org/policies
 * Copyright (c) 2017-2019, SyLabs, Inc. All rights reserved.
 * Copyright (c) 2017, SingularityWare, LLC. All rights reserved.
 *
 * This software is licensed under a 3-clause BSD license.  Please
 * consult LICENSE.md file distributed with the sources of this project regarding
 * your rights to use or distribute this software.
 *
 */


#define _GNU_SOURCE
#include <unistd.h>
#include <sys/syscall.h>

#include "include/capability.h"

int capget(cap_user_header_t hdrp, cap_user_data_t datap) {
    return syscall(__NR_capget, hdrp, datap);
}

int capset(cap_user_header_t hdrp, const cap_user_data_t datap) {
    return syscall(__NR_capset, hdrp, datap);
}
