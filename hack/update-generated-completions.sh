#!/bin/bash

# This script sets up a go workspace locally and builds all go components.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"

# ensure we have the latest compiled binaries
"${OS_ROOT}/hack/build-go.sh"

platform="$(os::build::host_platform)"
if [[ "${platform}" != "linux/amd64" ]]; then
  echo "WARNING: Generating completions on ${platform} may not be identical to running on linux/amd64 due to conditional compilation."
fi

#"${OS_ROOT}/hack/build-go.sh" tools/genbashcomp

OUTPUT_REL_DIR=${1:-""}
OUTPUT_DIR_ROOT="${OS_ROOT}/${OUTPUT_REL_DIR}/contrib/completions"

mkdir -p "${OUTPUT_DIR_ROOT}/bash" || echo $? > /dev/null
mkdir -p "${OUTPUT_DIR_ROOT}/zsh" || echo $? > /dev/null

os::build::gen-completions "${OUTPUT_DIR_ROOT}/bash" "bash"
os::build::gen-completions "${OUTPUT_DIR_ROOT}/zsh" "zsh"