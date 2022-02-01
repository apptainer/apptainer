#!/bin/sh
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
# Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.

for script in /.singularity.d/env/*.sh; do
    if [ -f "$script" ]; then
        . "$script"
    fi
done


if test -n "${SINGULARITY_APPNAME:-}"; then

    if test -x "/scif/apps/${SINGULARITY_APPNAME:-}/scif/test"; then
        exec "/scif/apps/${SINGULARITY_APPNAME:-}/scif/test" "$@"
    elif test -x "/scif/apps/singularity/scif/test"; then
        exec "/scif/apps/singularity/scif/test" "$@"
    else
        echo "No tests for contained app: ${SINGULARITY_APPNAME:-}"
        exit 1
    fi
elif test -x "/.singularity.d/test"; then
    exec "/.singularity.d/test" "$@"
else
    echo "No test found in container, executing /bin/sh -c true"
    exec /bin/sh -c true
fi
