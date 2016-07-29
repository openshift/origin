#!/bin/bash
#
# This library holds golang related utility functions.

# os::golang::verify_go_version ensure the go tool exists and is a viable version.
function os::golang::verify_go_version() {
	if ! which go &>/dev/null; then
		os::log::error "Can't find 'go' in PATH, please fix and retry."
		os::log::error "See http://golang.org/doc/install for installation instructions."
		return 1
	fi

	local go_version
	go_version=($(go version))
	if [[ "${go_version[2]}" != go1.6* ]]; then
		os::log::info "Detected go version: ${go_version[*]}."
		if [[ -z "${PERMISSIVE_GO:-}" ]]; then
			os::log::error "Please install Go version 1.6 or use PERMISSIVE_GO=y to bypass this check."
			return 1
		else
			os::log::warn "Detected golang version doesn't match preferred Go version for Origin."
			os::log::warn "This version mismatch could lead to differences in execution between this run and the Origin CI systems."
			return 0
		fi
	fi
}
readonly -f os::golang::verify_go_version

# os::golang::verify_golint_version ensure the golint tool exists.
function os::golang::verify_golint_version() {
	if ! which golint &>/dev/null; then
		os::log::error "Unable to detect 'golint' package."
		os::log::error "To install it, run: 'go get github.com/golang/lint/golint'."
		return 1
	fi
}
readonly -f os::golang::verify_golint_version
