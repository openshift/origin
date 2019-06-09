#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

readonly S2I_ROOT=$(dirname "${BASH_SOURCE}")/..

"${S2I_ROOT}/hack/test-docker.sh"
"${S2I_ROOT}/hack/test-dockerfile.sh"

