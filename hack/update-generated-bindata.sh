#!/bin/bash
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

EXAMPLES=examples
OUTPUT_PARENT=${OUTPUT_ROOT:-$OS_ROOT}

pushd vendor/github.com/jteeuwen/go-bindata > /dev/null
  go install ./...
popd > /dev/null
os::util::ensure::gopath_binary_exists 'go-bindata'

pushd "${OS_ROOT}" > /dev/null
"$(os::util::find::gopath_binary go-bindata)" \
    -nocompress \
    -nometadata \
    -prefix "bootstrap" \
    -pkg "bootstrap" \
    -o "${OUTPUT_PARENT}/pkg/bootstrap/bindata.go" \
    -ignore "README.md" \
    -ignore ".*\.go$" \
    -ignore "\.DS_Store" \
    -ignore application-template.json \
    ${EXAMPLES}/image-streams/... \
    ${EXAMPLES}/db-templates/... \
    ${EXAMPLES}/jenkins \
    ${EXAMPLES}/jenkins/pipeline \
    ${EXAMPLES}/quickstarts/... \
	${EXAMPLES}/logging/... \
	${EXAMPLES}/heapster/... \
	${EXAMPLES}/prometheus/... \
    pkg/image/admission/imagepolicy/api/v1/...

"$(os::util::find::gopath_binary go-bindata)" \
    -nocompress \
    -nometadata \
    -prefix "testextended" \
    -pkg "testdata" \
    -o "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" \
    -ignore "\.DS_Store" \
    -ignore ".*\.(go|md)$" \
    test/extended/testdata/... \
    test/integration/testdata \
    examples/db-templates \
    examples/image-streams \
    examples/sample-app \
    examples/hello-openshift \
    examples/jenkins/...

popd > /dev/null

# If you hit this, please reduce other tests instead of importing more
if [[ "$( cat "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" | wc -c )" -gt 650000 ]]; then
    echo "error: extended bindata is $( cat "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" | wc -c ) bytes, reduce the size of the import" 1>&2
    exit 1
fi

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
