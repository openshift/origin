#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/buildenv/common.sh"

platforms=(linux/amd64 $OS_CROSSPLATFORMS)
targets=("${compile_targets[@]}")

if [[ $# -gt 0 ]]; then
  targets=("$@")
fi

for platform in "${platforms[@]}"; do
  (
    # Subshell to contain these exports
    export GOOS=${platform%/*}
    export GOARCH=${platform##*/}

    os::build::make_binaries "${targets[@]}"
  )
done
