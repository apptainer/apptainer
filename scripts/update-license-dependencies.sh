#!/bin/sh

set -e
set -u

if [ -d "vendor" ]; then
  echo "Please remove vendor directory before running this script"
  exit 255
fi

if [ ! -f "go.mod" ]; then
  echo "This script must be called from the project root directory,"
  echo "i.e. as scripts/update-license-dependencise.sh"
  exit 255
fi

# Exclude ourselves, and github.com/vbauerster/mpb for which
# the tool is not working properly, because it uses UNLICENSE
exclude="github.com/apptainer/apptainer|github.com/vbauerster/mpb"

# Ensure a constant sort order
export LC_ALL=C

$(go env GOROOT)/bin/go run github.com/google/go-licenses@v1.6.0 csv ./... | grep -v -E "${exclude}" | sort -k3,3 -k1,1 -t, > LICENSE_DEPENDENCIES.csv

# Header for the markdown file
cat <<-'EOF' >LICENSE_DEPENDENCIES.md
# Dependency Licenses

This project uses a number of dependencies, in accordance with their own
license terms. These dependencies are managed via the project `go.mod`
and `go.sum` files, and included in a `vendor/` directory in our official
source tarballs.

A full build or package of Apptainer uses all dependencies listed below.
If you `import "github.com/apptainer/apptainer"` into your own project then
you may use a subset of them.

The dependencies and their licenses are as follows:

EOF

while IFS="," read -r dep url license; do
  {
    echo "## ${dep}"
    echo ""
    echo "**License:** ${license}"
    echo ""
  } >>LICENSE_DEPENDENCIES.md

  # go-licenses can't work out the web url for non-github projects.
  # Fall back to using the dependency URL as a project URL
  if [ "${url}" = "Unknown" ]; then
    echo "**Project URL:** <https://${dep}>" >>LICENSE_DEPENDENCIES.md
  else
    echo "**License URL:** <${url}>" >>LICENSE_DEPENDENCIES.md
  fi
  echo "" >>LICENSE_DEPENDENCIES.md
done <LICENSE_DEPENDENCIES.csv

# Add github.com/vbauerster/mpb manually
cat <<-'EOF' >>LICENSE_DEPENDENCIES.md
## github.com/vbauerster/mpb

**License:** The Unlicense

**License URL:** <https://github.com/vbauerster/mpb/blob/master/UNLICENSE>
EOF

# Clean up
rm LICENSE_DEPENDENCIES.csv
