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

if test -n "$SINGULARITY_SHELL" -a -x "$SINGULARITY_SHELL"; then
    exec $SINGULARITY_SHELL "$@"

    echo "ERROR: Failed running shell as defined by '\$SINGULARITY_SHELL'" 1>&2
    exit 1

elif test -x /bin/bash; then
    SHELL=/bin/bash
    PS1="Apptainer $APPTAINER_NAME:\\w> "
    export SHELL PS1
    exec /bin/bash --norc "$@"
elif test -x /bin/sh; then
    SHELL=/bin/sh
    export SHELL
    exec /bin/sh "$@"
else
    echo "ERROR: /bin/sh does not exist in container" 1>&2
fi
exit 1
