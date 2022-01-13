#!/bin/sh
# Copyright (c) 2021-2022 Apptainer a Series of LF Projects LLC
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
# Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.
#

if [ -n "${SING_USER_DEFINED_PREPEND_PATH:-}" ]; then
	PATH="${SING_USER_DEFINED_PREPEND_PATH}:${PATH}"
fi

if [ -n "${SING_USER_DEFINED_APPEND_PATH:-}" ]; then
	PATH="${PATH}:${SING_USER_DEFINED_APPEND_PATH}"
fi

if [ -n "${SING_USER_DEFINED_PATH:-}" ]; then
	PATH="${SING_USER_DEFINED_PATH}"
fi

unset SING_USER_DEFINED_PREPEND_PATH \
	  SING_USER_DEFINED_APPEND_PATH \
	  SING_USER_DEFINED_PATH

export PATH
