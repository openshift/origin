#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

pushd "${OS_ROOT}/assets" > /dev/null
  grunt build
popd > /dev/null

pushd "${OS_ROOT}" > /dev/null
  # Put each component in its own go package for compilation performance
  # Strip off the dist folder from each package to flatten the resulting directory structure
  # Force timestamps to unify, and mode to 493 (0755)
  Godeps/_workspace/bin/go-bindata -nocompress -nometadata -prefix "assets/dist"      -pkg "assets" -o "pkg/assets/bindata.go"      -ignore "\\.gitignore" assets/dist/...
  Godeps/_workspace/bin/go-bindata -nocompress -nometadata -prefix "assets/dist.java" -pkg "java"   -o "pkg/assets/java/bindata.go" -ignore "\\.gitignore" assets/dist.java/...
popd > /dev/null
