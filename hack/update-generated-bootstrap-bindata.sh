#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install

EXAMPLES=examples
OUTPUT_PARENT=${OUTPUT_ROOT:-$OS_ROOT}

if [[ -z "$( which go-bindata )" ]]; then
  pushd vendor/github.com/jteeuwen/go-bindata > /dev/null
    go install ./...
  popd > /dev/null
fi

pushd "${OS_ROOT}" > /dev/null
  "$(os::util::find-go-binary go-bindata)" -nocompress -nometadata -prefix "bootstrap" -pkg "bootstrap" \
                                   -o "${OUTPUT_PARENT}/pkg/bootstrap/bindata.go" -ignore "README.md" -ignore ".*\.go$" \
                                   ${EXAMPLES}/image-streams/... \
                                   ${EXAMPLES}/db-templates/... \
                                   ${EXAMPLES}/jenkins/pipeline/... \
                                   ${EXAMPLES}/quickstarts/... \
                                   pkg/image/admission/imagepolicy/api/v1/...
popd > /dev/null

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
