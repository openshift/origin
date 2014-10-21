#!/bin/bash

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
cd "${OS_ROOT}"

source "${OS_ROOT}/hack/common.sh"

readonly OS_TARGET="${OS_ROOT}/_output/build"
readonly OS_GO_PACKAGE=github.com/openshift/origin
readonly OS_RELEASES="${OS_ROOT}/_output/releases"

compile_targets=(
  cmd/openshift
)

mkdir -p "${OS_TARGET}"

if [[ ! -f "/os-build-image" ]]; then
  echo "WARNING: This script should be run in the os-build container image!" >&2
fi

if [[ -f "./os-version-defs" ]]; then
  source "./os-version-defs"
else
  echo "WARNING: No version information provided in build image"
  readonly OS_VERSION="${OS_VERSION:-unknown}"
  readonly OS_GITCOMMIT="${OS_GITCOMMIT:-unknown}"
fi


function os::build::make_binary() {
  local -r gopkg=$1
  local -r bin=${gopkg##*/}

  echo "+++ Building ${bin} for ${GOOS}/${GOARCH}"
  pushd "${OS_ROOT}" >/dev/null
  godep go build -ldflags "${OS_LD_FLAGS-}" -o "${ARCH_TARGET}/${bin}" "${gopkg}"
  popd >/dev/null
}

function os::build::make_binaries() {
  [[ $# -gt 0 ]] || {
    echo "!!! Internal error. os::build::make_binaries called with no targets."
  }

  local -a targets=("$@")
  local -a binaries=()
  local target
  for target in "${targets[@]}"; do
    binaries+=("${OS_GO_PACKAGE}/${target}")
  done

  ARCH_TARGET="${OS_TARGET}/${GOOS}/${GOARCH}"
  mkdir -p "${ARCH_TARGET}"

  local b
  for b in "${binaries[@]}"; do
    os::build::make_binary "$b"
  done

  mkdir -p "${OS_RELEASES}"
  readonly ARCHIVE_NAME="openshift-origin-${OS_VERSION}-${OS_GITCOMMIT}-${GOOS}-${GOARCH}.tar.gz"
  tar -czf "${OS_RELEASES}/${ARCHIVE_NAME}" -C "${ARCH_TARGET}" .
}