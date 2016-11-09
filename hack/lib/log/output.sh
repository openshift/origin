#!/bin/bash

# This file contains functions used for writing log messages
# to stdout and stderr from scripts while they run.

# os::log::info writes the message to stdout.
#
# Arguments:
#  - all: message to write
function os::log::info() {
	echo "[INFO] $*"
}
readonly -f os::log::info

# os::log::warn writes the message to stderr.
# A warning indicates something went wrong but
# not so wrong that we cannot recover.
#
# Arguments:
#  - all: message to write
function os::log::warn() {
	os::text::print_yellow "[WARNING] $*" 1>&2
}
readonly -f os::log::warn

# os::log::error writes the message to stderr.
# An error indicates that something went wrong
# and we will most likely fail after this.
#
# Arguments:
#  - all: message to write
function os::log::error() {
	os::text::print_red "[ERROR] $*" 1>&2
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
	os::text::print_red "[FATAL] $*" 1>&2
	return 1
}
readonly -f os::log::fatal