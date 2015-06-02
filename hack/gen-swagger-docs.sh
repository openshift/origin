#!/bin/bash

# Script to generate docs from the latest swagger spec.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

pushd "${OS_ROOT}/hack/swagger-doc" > /dev/null
gradle gendocs --info
popd > /dev/null

echo "SUCCESS"