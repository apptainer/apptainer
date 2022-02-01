#!/usr/bin/env sh
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
#
# attempt to find all sh/bash files in source directories
#

get_shell_shebang_files() {
  grep -rm1 ^ -- * | grep -vE "^vendor|^builddir" | grep -E ':#!.*bin.*sh( |$)' | sed -e 's|:#!.*||'
}

get_shell_suffix_files() {
  find . -type f -name "*.sh" | sed -e 's|^\./||' | grep -vE '^vendor|^builddir'
}

get_all_unique_files() {
  (get_shell_shebang_files && get_shell_suffix_files) | sort -u
}

get_all_unique_files "$@"
