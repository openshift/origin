#!/bin/bash

set -e

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

GO_VERSION=($(go version))
echo "Detected go version: $(go version)"

go get golang.org/x/tools/cmd/cover github.com/tools/godep golang.org/x/tools/cmd/vet

# Check out a stable commit for go vet in order to version lock it to something we can work with
pushd $GOPATH/src/golang.org/x/tools >/dev/null 2>&1
  git fetch
  git checkout c262de870b618eed648983aa994b03bc04641c72 
popd >/dev/null 2>&1

# Re-install using this version of the tool
go install golang.org/x/tools/cmd/vet


ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
