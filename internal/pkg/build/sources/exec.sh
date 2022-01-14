#!/bin/sh
# Copyright (c) 2021-2022 Apptainer a Series of LF Projects LLC
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
# Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.

for script in $(ls /.singularity.d/env/*.sh | sort -n); do
    if [ -f "$script" ]; then
        . "$script"
    fi
done

exec "$@"
