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
        -o -wholename '*/output/*' \
      \) -prune \
    \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/%s\n"
}

# -covermode=atomic becomes default with -race in Go >=1.3
KUBE_COVER=${KUBE_COVER:--cover -covermode=atomic}
KUBE_TIMEOUT=${KUBE_TIMEOUT:--timeout 30s}

cd "${OS_TARGET}"

if [ "$1" != "" ]; then
  go test -race $KUBE_TIMEOUT $KUBE_COVER -coverprofile=tmp.out "$OS_GO_PACKAGE/$1" "${@:2}"
  exit 0
fi

find_test_dirs | xargs go test -race -timeout 30s $KUBE_COVER "${@:2}"
