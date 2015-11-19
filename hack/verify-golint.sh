#!/bin/bash

set -o errexit
set -o pipefail

if ! which golint &>/dev/null; then
  echo "Unable to detect 'golint' package"
  echo "To install it, run: 'go get github.com/golang/lint/golint'"
  exit 1
fi

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.4') ]]; then
  echo "Unknown go version '${GO_VERSION}', skipping golint."
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

cd "${OS_ROOT}"

arg="${1:-""}"
bad_files=""

if [ "$arg" == "-m" ]; then
  head=$(git rev-parse --short HEAD | xargs echo -n)
  set +e
  modified_files=$(git diff-tree --no-commit-id --name-only -r master..$head | \
    grep "^pkg" | grep ".go$" | grep -v "bindata.go$" | grep -v "Godeps" | \
    grep -v "third_party")
  if [ -n "${modified_files}" ]; then
    echo -e "Checking modified files: ${modified_files}\n"
    for f in $modified_files; do golint $f; done
    echo
  fi
  set -e
else
  bad_files=$(find_files | 
                sort -u | 
                sed 's/^.{2}//' | 
                xargs -n1 printf "${GOPATH}/src/${OS_GO_PACKAGE}/%s\n" | 
                xargs -n1 golint)
fi

if [[ -n "${bad_files}" ]]; then
  echo "golint detected following problems:"
  echo "${bad_files}"
  exit 1
fi
