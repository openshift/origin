#!/bin/bash

# See HACKING.md for usage

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

PRINT_PACKAGES=${PRINT_PACKAGES:-""}
TEST_KUBE=${TEST_KUBE:-""}
KUBE_GODEP_PATH="./Godeps/_workspace/src/k8s.io/kubernetes/pkg"

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
        -o -wholename './openshift.local.*' \
      \) -prune \
    \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/%s\n"
}

special_upstream_test_dirs() {
  echo "${OS_GO_PACKAGE}/${KUBE_GODEP_PATH}/api"
  echo "${OS_GO_PACKAGE}/${KUBE_GODEP_PATH}/api/v1"
}

# find the upstream test directories, excluding special-case directories and the upstream runtime package.
# The tests for the upstream runtime package are not solvent currently due to a patch for:
# https://github.com/GoogleCloudPlatform/kubernetes/pull/9971
find_upstream_test_dirs() {
  cd "${OS_ROOT}"
  find ./Godeps/_workspace/src/k8s.io/kubernetes -not \( \
      \( \
        -wholename "${KUBE_GODEP_PATH}/api" \
        -o -wholename "${KUBE_GODEP_PATH}/api/v1" \
        -o -wholename './Godeps/_workspace/src/k8s.io/kubernetes/pkg/runtime' \
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
elif [ -n "$TEST_KUBE" ]; then
  test_packages=`find_test_dirs; special_upstream_test_dirs; find_upstream_test_dirs`
else
  test_packages=`find_test_dirs; special_upstream_test_dirs`
fi

if [ -n "$PRINT_PACKAGES" ]; then
  for package in $test_packages
  do
    echo $package
  done

  exit 0
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

  echo 'mode: atomic' > ${OUTPUT_COVERAGE}/profiles.out
  find $OUTPUT_COVERAGE -name profile.out | xargs sed '/^mode: atomic$/d' >> ${OUTPUT_COVERAGE}/profiles.out
  go tool cover "-html=${OUTPUT_COVERAGE}/profiles.out" -o "${OUTPUT_COVERAGE}/coverage.html"

  rm -rf $OUTPUT_COVERAGE/$OS_GO_PACKAGE
else
  go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER "${@:2}" $test_packages
fi
