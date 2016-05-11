#!/bin/bash

# This script sets up a go workspace locally and invokes pythia to introspect code
# see: https://github.com/fzipp/pythia

# Prereq:
# go get github.com/fzipp/pythia

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Check for `go` binary and set ${GOPATH}.
os::build::setup_env

pythia github.com/openshift/origin/cmd/openshift
