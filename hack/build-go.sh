#!/bin/bash

# This script sets up a go workspace locally and builds all go components.

set -e

# Update the version.
$(dirname $0)/version-gen.sh

source $(dirname $0)/config-go.sh

cd "${OS_TARGET}"

BINARIES="cmd/openshift"

if [ $# -gt 0 ]; then
  BINARIES="$@"
fi

go install $(for b in $BINARIES; do echo "${OS_GO_PACKAGE}"/${b}; done)
