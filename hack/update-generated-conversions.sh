#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

# Do not allow a synthetic GOPATH for these checks
os::build::setup_env

"${OS_ROOT}/hack/build-go.sh" tools/genconversion
genconversion="$( os::build::find-binary genconversion )"

if [[ -z "${genconversion}" ]]; then
	echo "It looks as if you don't have a compiled genconversion binary."
	echo
	echo "If you are running from a clone of the git repo, please run"
	echo "'./hack/build-go.sh tools/genconversion'."
	exit 1
fi

${genconversion} --output-base="${GOPATH}/src" "$@"