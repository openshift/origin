#!/bin/bash

# This script sets up a go workspace locally and builds all go components.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

"${OS_ROOT}/hack/build-go.sh" tools/gendocs

# Find binary
gendocs="$(os::build::find-binary gendocs)"

if [[ -z "$gendocs" ]]; then
  {
    echo "It looks as if you don't have a compiled gendocs binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh tools/gendocs'."
  } >&2
  exit 1
fi

OUTPUT_DIR_REL=${1:-""}
OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/generated"
mkdir -p "${OUTPUT_DIR}" || echo $? > /dev/null
os::build::gen-docs "${gendocs}" "${OUTPUT_DIR}"
