/*
 * Copyright (c) Contributors to the Apptainer project, established as
 *   Apptainer a Series of LF Projects LLC.
 *   For website terms of use, trademark policy, privacy policy and other
 *   project policies see https://lfprojects.org/policies
 * Copyright (c) 2017-2019, SyLabs, Inc. All rights reserved.
 * Copyright (c) 2017, SingularityWare, LLC. All rights reserved.
 * 
 * This file is part of the Apptainer Linux container project. It is subject to the license
 * terms in the LICENSE.md file found in the top-level directory of this distribution and
 * at https://github.com/apptainer/apptainer/blob/main/LICENSE.md. No part
 * of Apptainer, including this file, may be copied, modified, propagated, or distributed
 * except according to the terms contained in the LICENSE.md file.
 * 
*/


#define _GNU_SOURCE

#include <unistd.h>
#include <errno.h>
#include <sys/syscall.h>
#include "include/setns.h"
#include "include/message.h"

#ifdef __NR_setns

int xsetns(int fd, int nstype) {
    return syscall(__NR_setns, fd, nstype);
}

#else

int xsetns(int fd, int nstype) {
    (void)fd;
    (void)nstype;
    apptainer_message(WARNING, "setns() not supported at compile time by kernel at time of building\n");
    errno = ENOSYS;
    return -1;
}

#endif /* __NR_setns */
