#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

govet_blacklist=( "${OS_GOVET_BLACKLIST[@]-}" )

function govet_blacklist_contains() {
	local text=$1
	for blacklist_entry in "${govet_blacklist[@]-}"; do
		if grep -Eqx "${blacklist_entry}" <<<"${text}"; then
			# the text we got matches this blacklist entry
			return 0
		fi
	done
	return 1
}

for test_dir in $(os::util::list_go_src_dirs); do
	if ! result="$(go tool vet -shadow=false -printfuncs=Info,Infof,Warning,Warningf "${test_dir}" 2>&1)"; then
		while read -r line; do
			if ! govet_blacklist_contains "${line}"; then
				echo "${line}"
				FAILURE=true
			fi
		done <<<"${result}"
	fi
done

# We don't want to exit on the first failure of go vet, so just keep track of
# whether a failure occurred or not.
if [[ -n "${FAILURE:-}" ]]; then
	os::log::fatal "FAILURE: go vet failed!"
fi
