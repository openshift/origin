#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::system_binary_exists 'golint'

arg="${1:-""}"
bad_files=""

if [ "$arg" == "-m" ]; then
	head=$(git rev-parse --short HEAD | xargs echo -n)
	set +e
	modified_files=$(git diff-tree --no-commit-id --name-only -r master..$head | \
		grep "^pkg" | grep ".go$" | grep -v "bindata.go$" | grep -v "Godeps" | \
		grep -v "third_party")
	if [ -n "${modified_files}" ]; then
		echo -e "Checking modified files: ${modified_files}\n"
		for f in $modified_files; do golint $f; done
		echo
	fi
	set -e
else
	bad_files=$(os::util::list_go_src_files | \
		sort -u | \
		sed 's/^.{2}//' | \
		xargs -n1 printf "${GOPATH}/src/${OS_GO_PACKAGE}/%s\n" | \
		xargs -n1 golint)
fi

if [[ -n "${bad_files}" ]]; then
	echo "golint detected following problems:"
	echo "${bad_files}"
	exit 1
fi
