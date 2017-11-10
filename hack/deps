#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

if [[ $# -eq 0 || ! -f "${OS_OUTPUT}/deps" ]]; then
  echo "Generating dependency graph ..." 1>&2
  mkdir -p "${OS_OUTPUT}"
  os::util::list_go_deps > "${OS_OUTPUT}/deps"
fi

if [[ $# -eq 0 ]]; then
  echo "Dependencies generated to ${OS_OUTPUT}/deps"
  echo
  echo "Install digraph with: go get -u golang.org/x/tools/cmd/digraph"
  echo 
  echo "To see the list of all dependencies of a package: "
  echo "  hack/deps.sh forward ${OS_GO_PACKAGE}/cmd/openshift"
  echo 
  echo "To see how a package was included into a binary (one particular way): "
  echo "  hack/deps.sh somepath ${OS_GO_PACKAGE}/cmd/openshift FULL_PACKAGE_NAME"
  exit 0
fi

os::util::ensure::system_binary_exists 'digraph'
cat "${OS_OUTPUT}/deps" | digraph $@
