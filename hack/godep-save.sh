#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

pin-godep() {
  pushd "${GOPATH}/src/github.com/tools/godep" > /dev/null
    git checkout "$1"
    "${GODEP}" go install
  popd > /dev/null
}

# build the godep tool
# Again go get stinks, hence || true
go get -u github.com/tools/godep 2>/dev/null || true
GODEP="${GOPATH}/bin/godep"

# Use to following if we ever need to pin godep to a specific version again
pin-godep 'v75'

# Some things we want in godeps aren't code dependencies, so ./...
# won't pick them up.
REQUIRED_BINS=(
  "github.com/elazarl/goproxy"
  "github.com/golang/mock/gomock"
  "github.com/containernetworking/cni/plugins/ipam/host-local"
  "github.com/containernetworking/cni/plugins/main/loopback"
  "k8s.io/kubernetes/cmd/libs/go2idl/go-to-protobuf/protoc-gen-gogo"
  "k8s.io/kubernetes/cmd/libs/go2idl/client-gen"
  "github.com/onsi/ginkgo/ginkgo"
  "github.com/jteeuwen/go-bindata/go-bindata"
  "./..."
)

"${GODEP}" save -t "${REQUIRED_BINS[@]}"
