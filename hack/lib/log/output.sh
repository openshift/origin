#!/bin/bash

# This file contains functions used for writing log messages
# to stdout and stderr from scripts while they run.

# os::log::info writes the message to stdout.
#
# Arguments:
#  - all: message to write
function os::log::info() {
	os::log::internal::prefix_lines "[INFO]" "$*"
}
readonly -f os::log::info

# os::log::warn writes the message to stderr.
# A warning indicates something went wrong but
# not so wrong that we cannot recover.
#
# Arguments:
#  - all: message to write
function os::log::warn() {
	os::text::print_yellow "$( os::log::internal::prefix_lines "[WARNING]" "$*" )" 1>&2
}
readonly -f os::log::warn

# os::log::error writes the message to stderr.
# An error indicates that something went wrong
# and we will most likely fail after this.
#
# Arguments:
#  - all: message to write
function os::log::error() {
	os::text::print_red "$( os::log::internal::prefix_lines "[ERROR]" "$*" )" 1>&2
}
readonly -f os::log::error

# os::log::fatal writes the message to stderr and
# returns a non-zero code to force a process exit.
# A fatal error indicates that there is no chance
# of recovery.
#
# Arguments:
#  - all: message to write
function os::log::fatal() {
	os::text::print_red "$( os::log::internal::prefix_lines "[FATAL]" "$*" )" 1>&2
	exit 1
}
readonly -f os::log::fatal

# os::log::debug writes the message to stderr if
# the ${OS_DEBUG} variable is set.
#
# Arguments:
#  - all: message to write
function os::log::debug() {
	if [[ -n "${OS_DEBUG:-}" ]]; then
		os::text::print_blue "$( os::log::internal::prefix_lines "[DEBUG]" "$*" )" 1>&2
	fi
}
readonly -f os::log::debug

# os::log::internal::prefix_lines prints out the
# original content with the given prefix at the
# start of every line.
#
# Arguments:
#  - 1: prefix for lines
#  - 2: content to prefix
function os::log::internal::prefix_lines() {
	local prefix="$1"
	local content="$2"

	local old_ifs="${IFS}"
	IFS=$'\n'
	for line in ${content}; do
		echo "${prefix} ${line}"
	done
	IFS="${old_ifs}"
}