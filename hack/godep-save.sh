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

# fail early if we don't have a proper kube version from the latest git tag
function kube-version () {
    local git_v=$(cd "${GOPATH}/src/k8s.io/kubernetes" && git describe --tags)
    echo "${git_v##-*}"
}
if ! [[ "$(kube-version)" =~ v[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+ ]]; then
    echo "Unexpected kubernetes version: '$(kube-version)'. Check for non-release git tags that shouldn't be there." 1>&2
    exit 1
fi

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
  "k8s.io/kubernetes/pkg/api/testing/compat"
  "k8s.io/kubernetes/test/e2e/generated"
  "github.com/onsi/ginkgo/ginkgo"
  "github.com/jteeuwen/go-bindata/go-bindata"
  "./..."
)

TMPGOPATH=`mktemp -d`
trap "rm -rf $TMPGOPATH" EXIT
mkdir $TMPGOPATH/src

fork-without-vendor () {
  local PKG="$1"
  echo "Forking $PKG without vendor/"
  local DIR=$(dirname "$PKG")
  mkdir -p "$TMPGOPATH/src/$DIR"
  cp -a "$GOPATH/src/$PKG" "$TMPGOPATH/src/$DIR"
  pushd "$TMPGOPATH/src/$PKG" >/dev/null
    local OLDREV=$(git rev-parse HEAD)
    git rm -qrf vendor/
    git commit -q -m "Remove vendor/"
    local NEWREV=$(git rev-parse HEAD)
  popd >/dev/null
  echo "s/$NEWREV/$OLDREV/" >> "$TMPGOPATH/undo.sed"
}

fork-with-fake-packages () {
  local PKG="$1"
  shift
  echo "Forking $PKG with fake packages: $*"
  local DIR=$(dirname "$PKG")
  mkdir -p "$TMPGOPATH/src/$DIR"
  cp -a "$GOPATH/src/$PKG" "$TMPGOPATH/src/$DIR"
  pushd "$TMPGOPATH/src/$PKG" >/dev/null
    local OLDREV=$(git rev-parse HEAD)
    for FAKEPKG in "$@"; do
      if [ -n "$(ls $FAKEPKG/*.go 2>/dev/null)" ]; then
        echo "'fork::with::fake::packages $PKG $FAKEPKG' failed because $FAKEPKG already exists." 1>&2
        exit 1
      fi
      mkdir -p "$FAKEPKG"
      echo "package $(basename $DIR)" > "$FAKEPKG/doc.go"
      git add "$FAKEPKG/doc.go"
      echo "$PKG/$FAKEPKG" >> "$TMPGOPATH/fake-packages"
    done
    git commit -a -q -m "Add fake package $*"
    local NEWREV=$(git rev-parse HEAD)
  popd >/dev/null
  echo "s/$NEWREV/$OLDREV/" >> "$TMPGOPATH/undo.sed"
}

undo-forks-in-godeps-json () {
  echo "Replacing forked revisions with original revisions in Godeps.json"
  sed -f "$TMPGOPATH/undo.sed" Godeps/Godeps.json > "$TMPGOPATH/Godeps.json"
  mv "$TMPGOPATH/Godeps.json" Godeps/Godeps.json
}

godep-save () {
  echo "Deleting vendor/ and Godeps/"
  rm -rf vendor/ Godeps/
  echo "Running godep-save. This takes around 15 minutes."
  GOPATH=$TMPGOPATH:$GOPATH:$GOPATH/src/k8s.io/kubernetes/staging "${GODEP}" save "$@"

  undo-forks-in-godeps-json

  # godep fails to copy all package in staging because it gets confused with the symlinks. Moreover,
  # godep fails to copy dependencies of tests. Hence, we copy over manually until we have proper staging
  # repo tooling and we have replaced godep with something sane.
  rsync -ax --exclude='vendor/' --exclude="_output/" --exclude '*_test.go' --include='*.go' --include='*/' --exclude='*' $GOPATH/src/k8s.io/kubernetes/ vendor/k8s.io/kubernetes/

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

  # filter out fake packages
  for pkg in $(cat "$TMPGOPATH/fake-packages"); do
    re+="${sep}$pkg"
    sep="|"
  done

  # filter out staging repos from Godeps.json
  jq ".Deps |= map( select(.ImportPath | test(\"^(${re})\$\"; \"\") | not ) )" Godeps/Godeps.json > "$TMPGOPATH/Godeps.json"
  unexpand -t2 "$TMPGOPATH/Godeps.json" > Godeps/Godeps.json
}

missing-test-deps () {
  go list -f $'{{range .Imports}}{{.}}\n{{end}}{{range .TestImports}}{{.}}\n{{end}}{{range .XTestImports}}{{.}}\n{{end}}' ./vendor/k8s.io/kubernetes/... | grep '\.' | grep -v github.com/openshift/origin | sort -u || true
}

fork-without-vendor github.com/docker/distribution
fork-without-vendor github.com/libopenstorage/openstorage
fork-with-fake-packages github.com/docker/docker \
  api/types \
  api/types/blkiodev \
  api/types/container \
  api/types/filters \
  api/types/mount \
  api/types/network \
  api/types/registry \
  api/types/strslice \
  api/types/swarm \
  api/types/versions

# This is grotesque: godep-save does not copy dependencies of tests. Because we want to run the
# kubernetes tests, we have to extract the missing test dependencies and run godep-save again
# until we converge. Because we rsync kubernetes itself above, two iterations should be enough.
MISSING_TEST_DEPS=""
while true; do
  godep-save -t "${REQUIRED_BINS[@]}" ${MISSING_TEST_DEPS}
  NEW_MISSING_TEST_DEPS="$(missing-test-deps)"
  if [ -z "${NEW_MISSING_TEST_DEPS}" ]; then
    break
  fi
  echo "Missing dependencies for kubernetes tests found. Sorry, running godep-save again to get: ${NEW_MISSING_TEST_DEPS}"
  MISSING_TEST_DEPS+=" ${NEW_MISSING_TEST_DEPS}"
done

echo
echo "Do not forget to run hack/copy-kube-artifacts.sh"
