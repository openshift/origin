#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::golang::verify_go_version

bad_files=$(find_files | xargs gofmt -s -l)
if [[ -n "${bad_files}" ]]; then
	echo "!!! gofmt needs to be run on the following files: " >&2
	echo "${bad_files}"
	echo "Try running 'gofmt -s -d [path]'" >&2
	echo "Or autocorrect with 'hack/verify-gofmt.sh | xargs -n 1 gofmt -s -w'" >&2
	exit 1
fi
