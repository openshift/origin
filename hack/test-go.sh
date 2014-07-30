#!/bin/bash

set -e

source $(dirname $0)/config-go.sh


find_test_dirs() {
  (
    cd src/${OS_GO_PACKAGE}
    find . -not \( \
        \( \
          -wholename './third_party' \
          -o -wholename './release' \
          -o -wholename './target' \
          -o -wholename '*/third_party/*' \
          -o -wholename '*/output/*' \
        \) -prune \
      \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u
  )
}

# -covermode=atomic becomes default with -race in Go >=1.3
COVER="-cover -covermode=atomic -coverprofile=tmp.out"

cd "${OS_TARGET}"

if [ "$1" != "" ]; then
  go test -race -timeout 30s $COVER "$OS_GO_PACKAGE/$1" "${@:2}"
  exit 0
fi

for package in $(find_test_dirs); do
  go test -race -timeout 30s $COVER "${OS_GO_PACKAGE}/${package}" "${@:2}"
done
