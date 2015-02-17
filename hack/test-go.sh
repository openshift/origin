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
        -o -wholename './.git' \
        -o -wholename './assets/node_modules' \
      \) -prune \
    \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/%s\n"
}

# -covermode=atomic becomes default with -race in Go >=1.3
if [ -z ${KUBE_COVER+x} ]; then
  KUBE_COVER=""
fi

OUTPUT_COVERAGE=${OUTPUT_COVERAGE:-""}

if [ -n "${OUTPUT_COVERAGE}" ]; then
  if [ -z ${KUBE_RACE+x} ]; then
    KUBE_RACE="-race"
  fi
  if [ -z "${KUBE_COVER}" ]; then
    KUBE_COVER="-cover -covermode=atomic"
  fi
fi

if [ -z ${KUBE_RACE+x} ]; then
  KUBE_RACE=""
fi

KUBE_TIMEOUT=${KUBE_TIMEOUT:--timeout 60s}

if [ "${1-}" != "" ]; then
  test_packages="$OS_GO_PACKAGE/$1"
else
  test_packages=`find_test_dirs`
fi

export OPENSHIFT_ON_PANIC=crash

if [[ -n "${KUBE_COVER}" && -n "${OUTPUT_COVERAGE}" ]]; then
  # Iterate over packages to run coverage
  test_packages=( $test_packages )
  for test_package in "${test_packages[@]}"
  do
    mkdir -p "$OUTPUT_COVERAGE/$test_package"
    KUBE_COVER_PROFILE="-coverprofile=$OUTPUT_COVERAGE/$test_package/profile.out"

    go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER "$KUBE_COVER_PROFILE" "$test_package" "${@:2}"
  done

  for test_package in "${test_packages[@]}"
  do
    if [ -f "${OUTPUT_COVERAGE}/$test_package/profile.out" ]; then
      go tool cover "-html=${OUTPUT_COVERAGE}/$test_package/profile.out" -o "${OUTPUT_COVERAGE}/$test_package/coverage.html"
      echo "coverage: ${OUTPUT_COVERAGE}/$test_package/coverage.html"
    fi
  done
else
  go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER "${@:2}" $test_packages
fi
