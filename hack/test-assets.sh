#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS-}" == "true" && "${TEST_ASSETS-}" == "false" ]]; then
  exit
fi

pushd "${OS_ROOT}/assets" > /dev/null
  grunt test
  grunt build
popd > /dev/null

pushd "${OS_ROOT}" > /dev/null

  # Put each component in its own go package for compilation performance
  # Strip off the dist folder from each package to flatten the resulting directory structure
  Godeps/_workspace/bin/go-bindata -nocompress -nometadata -prefix "assets/dist"      -pkg "assets" -o "_output/test/assets/bindata.go"      -ignore "\\.gitignore" assets/dist/...
  Godeps/_workspace/bin/go-bindata -nocompress -nometadata -prefix "assets/dist.java" -pkg "java"   -o "_output/test/assets/java/bindata.go" -ignore "\\.gitignore" assets/dist.java/...

  echo "Validating checked in bindata.go is up to date..."
  if ! assetdiff=$(diff -u _output/test/assets/bindata.go pkg/assets/bindata.go) ; then

    echo "$assetdiff" | head -n 10

    pushd "${OS_ROOT}/assets" > /dev/null

      if [[ "${TRAVIS-}" == "true" ]]; then
        echo ""
        echo "Bower versions..."
        bower list -o

        echo ""
        echo "NPM versions..."
        npm list
      fi

    popd > /dev/null  

    exit 1
  fi

  echo "Validating checked in java/bindata.go is up to date..."
  if ! assetdiff=$(diff -u _output/test/assets/java/bindata.go pkg/assets/java/bindata.go) ; then
    echo "$assetdiff" | head -n 10
    exit 1
  fi

popd > /dev/null
