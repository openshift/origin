#!/bin/bash

# Script to generate docs from the latest swagger spec.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${OS_ROOT}"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
os::log::install_errexit

pushd "${OS_ROOT}/hack/swagger-doc" > /dev/null
gradle gendocs --info
popd > /dev/null

echo "[INFO] Swagger doc generation successful"
