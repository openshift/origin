#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# Register function to be called on EXIT to remove generated binary.
function cleanup {
  rm "${OS_ROOT}/images/service-catalog-proxy/service-catalog-proxy"
}
trap cleanup EXIT

pushd "${OS_ROOT}/images/service-catalog-proxy"
cp -v ../../_output/local/bin/linux/amd64/openshift service-catalog-proxy
docker build -t service-catalog-proxy:latest .
popd