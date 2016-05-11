#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

hack/build-go.sh tools/gendeepcopy
gendeepcopy="$( os::build::find-binary gendeepcopy )"

if [[ -z "${gendeepcopy}" ]]; then
	echo "It looks as if you don't have a compiled gendeepcopy binary."
	echo
	echo "If you are running from a clone of the git repo, please run"
	echo "'./hack/build-go.sh tools/gendeepcopy'."
	exit 1
fi

${gendeepcopy} --output-base="${OS_GOPATH}/src" "$@"