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


#ifndef __APPTAINER_CAPABILITY_H_
#define __APPTAINER_CAPABILITY_H_

#include <linux/capability.h>

/* 2.6.32 kernel is the minimal kernel version supported where latest cap is 33 */
#define CAPSET_MIN  33
/* 40 is the latest cap since kernel 5.9 */
#define CAPSET_MAX  40

/* Support only 64 bits sets, since kernel 2.6.26 */
#ifdef _LINUX_CAPABILITY_VERSION_3
#  define LINUX_CAPABILITY_VERSION  _LINUX_CAPABILITY_VERSION_3
#else
#  error Linux 64 bits capability set not supported
#endif /* _LINUX_CAPABILITY_VERSION_3 */

int capget(cap_user_header_t, cap_user_data_t);
int capset(cap_user_header_t, const cap_user_data_t);

#endif /* __APPTAINER_CAPABILITY_H_ */
