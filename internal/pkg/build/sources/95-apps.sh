#!/bin/sh
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
# Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult
# https://github.com/apptainer/apptainer/blob/main/LICENSE.md regarding your
# rights to use or distribute this software.
#
# Copyright (c) 2017, SingularityWare, LLC. All rights reserved.
#
# See the COPYRIGHT.md file at the top-level directory of this distribution and at
# https://github.com/apptainer/apptainer/blob/main/COPYRIGHT.md.
#
# This file is part of the Apptainer Linux container project. It is subject to the license
# terms in the LICENSE.md file found in the top-level directory of this distribution and
# at https://github.com/apptainer/apptainer/blob/main/LICENSE.md. No part
# of Apptainer, including this file, may be copied, modified, propagated, or distributed
# except according to the terms contained in the LICENSE.md file.

if test -n "${SINGULARITY_APPNAME:-}"; then

    # The active app should be exported
    export SINGULARITY_APPNAME

    if test -d "/scif/apps/${SINGULARITY_APPNAME:-}/"; then
        SCIF_APPS="/scif/apps"
        SCIF_APPROOT="/scif/apps/${SINGULARITY_APPNAME:-}"
        export SCIF_APPROOT SCIF_APPS
        PATH="/scif/apps/${SINGULARITY_APPNAME:-}:/scif/apps/singularity:$PATH"

        # Automatically add application bin to path
        if test -d "/scif/apps/${SINGULARITY_APPNAME:-}/bin"; then
            PATH="/scif/apps/${SINGULARITY_APPNAME:-}/bin:$PATH"
        elif test -d "/scif/apps/singularity/bin"; then
            PATH="/scif/apps/singularity/bin:$PATH"
        fi

        # Automatically add application lib to LD_LIBRARY_PATH
        if test -d "/scif/apps/${SINGULARITY_APPNAME:-}/lib"; then
            LD_LIBRARY_PATH="/scif/apps/${SINGULARITY_APPNAME:-}/lib:$LD_LIBRARY_PATH"
            export LD_LIBRARY_PATH
        elif test -d "/scif/apps/singularity/lib"; then
            LD_LIBRARY_PATH="/scif/apps/singularity/lib:$LD_LIBRARY_PATH"
            export LD_LIBRARY_PATH
        fi

        # Automatically source environment
        if [ -f "/scif/apps/${SINGULARITY_APPNAME:-}/scif/env/01-base.sh" ]; then
            . "/scif/apps/${SINGULARITY_APPNAME:-}/scif/env/01-base.sh"
		elif [ -f "/scif/apps/singularity/scif/env/01-base.sh" ]; then
            . "/scif/apps/singularity/scif/env/01-base.sh" "$@"
        fi
        if [ -f "/scif/apps/${SINGULARITY_APPNAME:-}/scif/env/90-environment.sh" ]; then
            . "/scif/apps/${SINGULARITY_APPNAME:-}/scif/env/90-environment.sh"
		elif [ -f "/scif/apps/singularity/scif/env/90-environment.sh" ]; then
            . "/scif/apps/singularity/scif/env/90-environment.sh" "$@"
        fi

        export PATH
    else
        echo "Could not locate the container application: ${SINGULARITY_APPNAME}"
        exit 1
    fi
fi
