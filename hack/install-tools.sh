#!/bin/bash

set -e

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

GO_VERSION=($(go version))
echo "Detected go version: $(go version)"

go get golang.org/x/tools/cmd/cover github.com/tools/godep golang.org/x/tools/cmd/vet

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
