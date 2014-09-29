#!/bin/bash

set -e

source $(dirname $0)/config-go.sh

find_test_dirs() {
  cd src/${OS_GO_PACKAGE}
  find . -not \( \
      \( \
        -wholename './third_party' \
        -wholename './Godeps' \
        -o -wholename './release' \
        -o -wholename './target' \
        -o -wholename '*/third_party/*' \
        -o -wholename '*/Godeps/*' \
        -o -wholename '*/_output/*' \
      \) -prune \
    \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/%s\n"
}

# there is currently a race in the coverage code in tip.  Remove this when it is fixed
# see https://code.google.com/p/go/issues/detail?id=8630 for details.
if [ "${TRAVIS_GO_VERSION}" == "tip" ]; then
  KUBE_COVER=""
else
  # -covermode=atomic becomes default with -race in Go >=1.3
  KUBE_COVER=${KUBE_COVER:--cover -covermode=atomic}
fi
KUBE_TIMEOUT=${KUBE_TIMEOUT:--timeout 30s}

# make race detection configurable to facilitate faster local iteration
if [ "${KUBE_RACE:-true}" == "false" ] ; then
  KUBE_RACE=""
else
  KUBE_RACE="-race"
fi

cd "${OS_TARGET}"

if [ "$1" != "" ]; then
  go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER -coverprofile=tmp.out "$OS_GO_PACKAGE/$1" "${@:2}"
  exit 0
fi

find_test_dirs | xargs go test $KUBE_RACE $KUBE_TIMEOUT $KUBE_COVER "${@:2}"
