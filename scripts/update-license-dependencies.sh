#!/bin/sh

set -e
set -u

go install github.com/google/go-licenses@v1.6.0

if [ -d "vendor" ]; then
  echo "Please remove vendor directory before running this script"
  exit 255
fi

if [ ! -f "go.mod" ]; then
  echo "This script must be called from the project root directory,"
  echo "i.e. as scripts/update-license-dependencise.sh"
  exit 255
fi

go-licenses report ./... --ignore github.com/apptainer/apptainer --template scripts/LICENSE_DEPENDENCIES.tpl > LICENSE_DEPENDENCIES.md
