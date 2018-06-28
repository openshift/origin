#!/bin/bash
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

OUTPUT_PARENT=${OUTPUT_ROOT:-$OS_ROOT}

pushd vendor/github.com/jteeuwen/go-bindata > /dev/null
  go install ./...
popd > /dev/null
os::util::ensure::gopath_binary_exists 'go-bindata'

pushd "${OS_ROOT}" > /dev/null
"$(os::util::find::gopath_binary go-bindata)" \
    -nocompress \
    -nometadata \
    -prefix "manifests" \
    -pkg "v310_00_assets" \
    -o "${OUTPUT_PARENT}/pkg/operator/v310_00_assets/bindata.go" \
    -ignore "OWNERS" \
    manifests/v3.10.0/...

popd > /dev/null

# If you hit this, please reduce other tests instead of importing more
if [[ "$( cat "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" | wc -c )" -gt 1500000 ]]; then
    echo "error: extended bindata is $( cat "${OUTPUT_PARENT}/test/extended/testdata/bindata.go" | wc -c ) bytes, reduce the size of the import" 1>&2
    exit 1
fi

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
