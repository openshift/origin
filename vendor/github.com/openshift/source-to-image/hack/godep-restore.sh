#!/bin/bash

# Sometimes godep needs 'other' remotes. So add those remotes
preload-remote() {
  local orig_org="$1"
  local orig_project="$2"
  local alt_org="$3"
  local alt_project="$4"

  # Go get stinks, which is why we have the || true...
  go get -d "${orig_org}/${orig_project}" &>/dev/null || true

  repo_dir="${GOPATH}/src/${orig_org}/${orig_project}"
  pushd "${repo_dir}" > /dev/null
    git remote add "${alt_org}-remote" "https://${alt_org}/${alt_project}.git" > /dev/null || true
    git remote update > /dev/null
  popd > /dev/null
}

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

# preload any odd-ball remotes
preload-remote "github.com/docker" "distribution" "github.com/openshift" "docker-distribution"

# fill out that nice clean place with the origin godeps
echo "Starting to download all godeps. This takes a while"

pushd "${GOPATH}/src/github.com/openshift/source-to-image" > /dev/null
  "${GODEP}" restore "$@"
popd > /dev/null

echo "Download finished into ${GOPATH}"
echo "######## REMEMBER ########"
echo "You cannot just godep save ./..."
echo "You should use hack/godep-save.sh"
echo "hack/godep-save.sh forces the inclusion of packages that godep alone will not include"
