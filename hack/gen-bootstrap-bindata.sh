#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

EXAMPLES=examples
OUTPUT_PARENT=${OUTPUT_ROOT:-$OS_ROOT}

pushd ${OS_ROOT}/Godeps/_workspace > /dev/null
  godep_path=$(pwd)
  pushd src/github.com/jteeuwen/go-bindata > /dev/null
    GOPATH=$godep_path go install ./...
  popd > /dev/null
popd > /dev/null

pushd "${OS_ROOT}" > /dev/null
  Godeps/_workspace/bin/go-bindata -nocompress -nometadata -prefix "bootstrap" -pkg "bootstrap" \
                                   -o "${OUTPUT_PARENT}/pkg/bootstrap/bindata.go" -ignore "README.md" \
                                   ${EXAMPLES}/image-streams/... \
                                   ${EXAMPLES}/db-templates/... \
                                   ${EXAMPLES}/jenkins/pipeline/... \
                                   ${EXAMPLES}/quickstarts/...
popd > /dev/null

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
