#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::golang::verify_go_version

bad_files=$(os::util::list_go_src_files | xargs gofmt -s -l)
if [[ -n "${bad_files}" ]]; then
	os::log::warning "!!! gofmt needs to be run on the listed files"
	echo "${bad_files}"
	os::log::fatal "Try running 'gofmt -s -d [path]'
Or autocorrect with 'hack/verify-gofmt.sh | xargs -n 1 gofmt -s -w'"
fi
