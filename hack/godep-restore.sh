#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

#### HACK ####
# Sometimes godep just can't handle things. This lets use manually put
# some deps in place first, so godep won't fall over.
preload-dep() {
  org="$1"
  project="$2"
  sha="$3"

  # Go get stinks, which is why we have the || true...
  go get -d "${org}/${project}" >/dev/null 2>&1 || true
  repo_dir="${GOPATH}/src/${org}/${project}"
  pushd "${repo_dir}" > /dev/null
    git remote update > /dev/null
    git checkout "${sha}" > /dev/null
  popd > /dev/null
}

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
preload-remote "github.com/openshift" "origin" "github.com/openshift" "origin" # this looks goofy, but if you are not in GOPATH you need to pull origin explicitly
preload-remote "k8s.io" "kubernetes" "github.com/openshift" "kubernetes"
preload-remote "github.com/docker" "distribution" "github.com/openshift" "docker-distribution"
preload-remote "github.com/skynetservices" "skydns" "github.com/openshift" "skydns"
preload-remote "github.com/coreos" "etcd" "github.com/openshift" "etcd"
preload-remote "github.com/emicklei" "go-restful" "github.com/openshift" "go-restful"
preload-remote "github.com/cloudflare" "cfssl" "github.com/openshift" "cfssl"
preload-remote "github.com/google" "certificate-transparency" "github.com/openshift" "certificate-transparency"
preload-remote "github.com/google" "cadvisor" "github.com/openshift" "cadvisor"

# preload any odd-ball commits
# kube e2e test dep
preload-dep "github.com/elazarl"     "goproxy" "$( go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "github.com/elazarl/goproxy" )"
# rkt test dep
preload-dep "github.com/golang/mock" "gomock"  "$( go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "github.com/golang/mock/gomock" )"
# docker storage dep
preload-dep "google.golang.org" "cloud"  "$( go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "google.golang.org/cloud" )"
preload-dep "github.com/karlseguin" "ccache" "master"

# fill out that nice clean place with the origin godeps
echo "Starting to download all godeps. This takes a while"

pushd "${GOPATH}/src/github.com/openshift/origin" > /dev/null
  GOPATH=$GOPATH:${PWD}/vendor/k8s.io/kubernetes/staging "${GODEP}" restore "$@"
popd > /dev/null

echo "Download finished into ${GOPATH}"
echo "######## REMEMBER ########"
echo "You cannot just godep save ./..."
echo "You should use hack/godep-save.sh"
echo "hack/godep-save.sh forces the inclusion of packages that godep alone will not include"
echo "Watch out for UPSTREAM patches when updating deps"
