#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

pin-godep() {
  pushd "${GOPATH}/src/github.com/tools/godep" > /dev/null
    git checkout "$1"
    "${GODEP}" go install
  popd > /dev/null
}

# fail early without jq
os::util::ensure::system_binary_exists "jq"

# fail early if any of the staging dirs is checked out
for pkg in "$GOPATH/src/k8s.io/kubernetes/staging/src/k8s.io/"*; do
  dir=$(basename $pkg)
  if [ -d "$GOPATH/src/k8s.io/$dir" ]; then
    echo "Conflicting $GOPATH/src/k8s.io/$dir found. Please remove from GOPATH." 1>&2
    exit 1
  fi
done

# build the godep tool
# Again go get stinks, hence || true
go get -u github.com/tools/godep 2>/dev/null || true
GODEP="${GOPATH}/bin/godep"

# Use to following if we ever need to pin godep to a specific version again
pin-godep 'v79'

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

GOPATH=$TMPGOPATH:$GOPATH:$GOPATH/src/k8s.io/kubernetes/staging "${GODEP}" save -t "${REQUIRED_BINS[@]}"

# godep fails to copy all package in staging because it gets confused with the symlinks.
# Hence, we copy over manually until we have proper staging repo tooling.
rsync -ax --include='*.go' --include='*/' --exclude='*' $GOPATH/src/k8s.io/kubernetes/staging/src/* vendor/k8s.io/kubernetes/staging/src/

# recreate symlinks
re=""
sep=""
for pkg in vendor/k8s.io/kubernetes/staging/src/k8s.io/*; do
  dir=$(basename $pkg)
  rm -rf vendor/k8s.io/$dir
  ln -s kubernetes/staging/src/k8s.io/$dir vendor/k8s.io/$dir

  # create regex for jq further down
  re+="${sep}k8s.io/$dir"
  sep="|"
done

# filter out staging repos from Godeps.json
jq ".Deps |= map( select(.ImportPath | test(\"^(${re})\$\"; \"\") | not ) )" Godeps/Godeps.json > "$TMPGOPATH/Godeps.json"
unexpand -t2 "$TMPGOPATH/Godeps.json" > Godeps/Godeps.json
