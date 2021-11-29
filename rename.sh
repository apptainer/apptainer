#!/usr/bin/env bash
#

function renameLC() {
    local path="${1}"
    local newpath="$(sed -e 's|singularity|apptainer|' <<< "${path}" )"
    printf 'git mv %s %s\n' "${path}" "${newpath}"
}

function renameUC() {
    local path="${1}"
    local newpath="$(sed -e 's|Singularity|Apptainer|' <<< "${path}" )"
    printf 'git mv %s %s\n' "${path}" "${newpath}"
}

export renameLC
export renameUC

function rename() {
    for file in $(find . -type f |grep singularity) ; do
        renameLC "${file}"
    done
    for file in $(find . -type f |grep Singularity) ; do
        renameUC "${file}"
    done
}

rename
