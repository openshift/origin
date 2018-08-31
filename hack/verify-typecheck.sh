#!/usr/bin/env bash
# Usage:
#
# To typecheck all the architectures run:
# $ ./hack/verify-typecheck.sh
#
# Additionally, to typecheck only e.g. linux/amd64:
# $ PLATFORMS=linux/amd64 ./hack/verify-typecheck.sh
#
# PLATFORMS is a string containing comma separated list of architectures to run typecheck for.
# see `./vendor/k8s.io/kubernetes/test/typecheck/main.go -h` for detailed list of arguments

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib/init.sh"

os::golang::verify_go_version

platforms=${PLATFORMS:-'linux/amd64,darwin/amd64,linux/arm,linux/386,linux/arm64,linux/ppc64le,linux/s390x,darwin/386'}

if ! go run ./vendor/k8s.io/kubernetes/test/typecheck/main.go -platform="${platforms}" "$@"; then
  os::log::fatal "Type Check has failed. This may cause cross platform build failures.
Please see https://git.k8s.io/kubernetes/test/typecheck for more information."
fi
