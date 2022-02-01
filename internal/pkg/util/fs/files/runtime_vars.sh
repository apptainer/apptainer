#!/bin/sh
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
# Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.


if test -n "${SING_USER_DEFINED_PREPEND_PATH:-}"; then
    PATH="${SING_USER_DEFINED_PREPEND_PATH}:${PATH}"
    unset SING_USER_DEFINED_PREPEND_PATH
fi

if test -n "${SING_USER_DEFINED_APPEND_PATH:-}"; then
    PATH="${PATH}:${SING_USER_DEFINED_APPEND_PATH}"
    unset SING_USER_DEFINED_APPEND_PATH
fi

if test -n "${SING_USER_DEFINED_PATH:-}"; then
    PATH="${SING_USER_DEFINED_PATH}"
    unset SING_USER_DEFINED_PATH
fi

export PATH
