#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

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