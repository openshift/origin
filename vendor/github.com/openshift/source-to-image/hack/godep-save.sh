#!/bin/bash

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
pin-godep 'v79'

godep-save () {
  echo "Deleting vendor/ and Godeps/"
  rm -rf vendor/ Godeps/
  echo "Running godep-save. This takes around 5 minutes."
  "${GODEP}" save "$@"
}

missing-test-deps () {
  go list -f $'{{range .Imports}}{{.}}\n{{end}}{{range .TestImports}}{{.}}\n{{end}}{{range .XTestImports}}{{.}}\n{{end}}' ./vendor/k8s.io/kubernetes/... | grep '\.' | grep -v github.com/openshift/origin | sort -u || true
}

godep-save -t ./...
