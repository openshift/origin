#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"

"${OS_ROOT}/hack/build-go.sh" tools/rebasehelpers/godepchecker

# Find binary
godepchecker="$(os::build::find-binary godepchecker)"
$godepchecker "$@"
