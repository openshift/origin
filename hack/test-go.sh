#!/bin/bash

# See HACKING.md for usage

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

find_test_dirs() {
  cd "${OS_ROOT}"
  find . -not \( \
      \( \
        -wholename './Godeps' \
        -o -wholename './release' \
        -o -wholename './target' \
        -o -wholename '*/Godeps/*' \
        -o -wholename '*/_output/*' \
      \) -prune \
    \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/%s\n"
}

# there is currently a race in the coverage code in tip.  Remove this when it is fixed
# see https://code.google.com/p/go/issues/detail?id=8630 for details.
if [ "${TRAVIS_GO_VERSION-}" == "tip" ]; then
  KUBE_COVER=""
else
  # -covermode=atomic becomes default with -race in Go >=1.3
  if [ -z ${KUBE_COVER+x} ]; then
    KUBE_COVER="-cover -covermode=atomic"
  fi
fi
KUBE_TIMEOUT=${KUBE_TIMEOUT:--timeout 45s}

if [ -z ${KUBE_RACE+x} ]; then
  KUBE_RACE="-race"
fi

if [ "${1-}" != "" ]; then
  test_packages="$OS_GO_PACKAGE/$1"
else
  test_packages=`find_test_dirs`
fi

OUTPUT_COVERAGE=${OUTPUT_COVERAGE:-""}

if [[ -n "${KUBE_COVER}" && -n "${OUTPUT_COVERAGE}" ]]; then
  # Iterate over packages to run coverage
  test_packages=( $test_packages )
  for test_package in "${test_packages[@]}"
  do
    mkdir -p "$OUTPUT_COVERAGE/$test_package"
    KUBE_COVER_PROFILE="-coverprofile=$OUTPUT_COVERAGE/$test_package/profile.out"

    go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER "$KUBE_COVER_PROFILE" "$test_package" "${@:2}"

    if [ -f "${OUTPUT_COVERAGE}/$test_package/profile.out" ]; then
      go tool cover "-html=${OUTPUT_COVERAGE}/$test_package/profile.out" -o "${OUTPUT_COVERAGE}/$test_package/coverage.html"
      echo "coverage: ${OUTPUT_COVERAGE}/$test_package/coverage.html"
    fi
  done
else
  go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER "${@:2}" $test_packages
fi
