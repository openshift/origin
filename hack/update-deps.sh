#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

cd "$(dirname "${BASH_SOURCE}")/.."
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::golang::verify_glide_version

# fail early if any of the staging dirs is checked out
for pkg in "$GOPATH/src/k8s.io/kubernetes/staging/src/k8s.io/"*; do
  dir=$(basename $pkg)
  if [ -d "$GOPATH/src/k8s.io/$dir" ]; then
    echo "Conflicting $GOPATH/src/k8s.io/$dir found. Please remove from GOPATH." 1>&2
    exit 1
  fi
done

# Some things we want in godeps aren't code dependencies, so ./...
# won't pick them up.
# TODO seems like this should be failing something somewhere
#REQUIRED_BINS=(
#  "github.com/elazarl/goproxy"
#  "github.com/golang/mock/gomock"
#  "github.com/containernetworking/cni/plugins/ipam/host-local"
#  "github.com/containernetworking/cni/plugins/main/loopback"
#  "k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo"
#  "k8s.io/code-generator/cmd/client-gen"
#  "k8s.io/kubernetes/pkg/api/testing/compat"
#  "k8s.io/kubernetes/test/e2e/generated"
#  "github.com/onsi/ginkgo/ginkgo"
#  "github.com/jteeuwen/go-bindata/go-bindata"
#)

glide update --strip-vendor

# recreate symlinks after vendoring
for pkg in vendor/k8s.io/kubernetes/staging/src/k8s.io/*; do
  dir=$(basename $pkg)
  rm -rf vendor/k8s.io/$dir
  ln -s kubernetes/staging/src/k8s.io/$dir vendor/k8s.io/$dir
done
