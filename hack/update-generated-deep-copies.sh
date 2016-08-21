#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

"${OS_ROOT}/hack/build-go.sh" tools/gendeepcopy
gendeepcopy="$( os::build::find-binary gendeepcopy )"

if [[ -z "${gendeepcopy}" ]]; then
	echo "It looks as if you don't have a compiled gendeepcopy binary."
	echo
	echo "If you are running from a clone of the git repo, please run"
	echo "'./hack/build-go.sh tools/gendeepcopy'."
	exit 1
fi

${gendeepcopy} --output-base="${GOPATH}/src" "$@"