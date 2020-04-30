#!/usr/bin/env bash
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

go generate ./test/extended

os::build::setup_env

OUTPUT_PARENT=${OUTPUT_ROOT:-$OS_ROOT}

pushd vendor/github.com/go-bindata/go-bindata > /dev/null
  go install ./...
popd > /dev/null
os::util::ensure::gopath_binary_exists 'go-bindata'

pushd "${OS_ROOT}" > /dev/null

"$(os::util::find::gopath_binary go-bindata)" \
    -nocompress \
    -nometadata \
    -prefix "testextended" \
    -pkg "testdata" \
    -o "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" \
    -ignore "OWNERS" \
    -ignore "\.DS_Store" \
    -ignore ".*\.(go|md)$" \
    -ignore "prometheus-standalone.yaml" \
    -ignore "node-exporter.yaml" \
    examples/db-templates \
	examples/image-streams \
	examples/sample-app \
	examples/quickstarts/... \
	examples/hello-openshift \
	examples/jenkins/... \
	examples/quickstarts/cakephp-mysql.json \
    test/extended/testdata/... \
    test/integration/testdata

popd > /dev/null

# If you hit this, please reduce other tests instead of importing more
if [[ "$( cat "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" | wc -c )" -gt 2500000 ]]; then
    echo "error: extended bindata is $( cat "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" | wc -c ) bytes, reduce the size of the import" 1>&2
    exit 1
fi

pushd "${OS_ROOT}/vendor/k8s.io/kubernetes" > /dev/null
PATH="$(dirname "$(os::util::find::gopath_binary go-bindata)"):${PATH}" SKIP_INSTALL_GO_BINDATA=y hack/generate-bindata.sh
popd > /dev/null

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
