#!/bin/bash
#
# This library holds miscellaneous utility functions. If there begin to be groups of functions in this
# file that share intent or are thematically similar, they should be split into their own files.

# os::util::describe_return_code describes an exit code
#
# Globals:
#  - START_TIME
# Arguments:
#  - 1: exit code to describe
# Returns:
#  None
function os::util::describe_return_code() {
	local return_code=$1

	if [[ "${return_code}" = "0" ]]; then
		echo -n "[INFO] $0 succeeded "
	else
		echo -n "[ERROR] $0 failed "
	fi

	if [[ -n "${START_TIME:-}" ]]; then
		local end_time="$(date +%s)"
		local elapsed_time="$(( ${end_time} - ${START_TIME} ))"
		echo "after ${elapsed_time}s"
	else
		echo
	fi
}

# os::util::install_describe_return_code installs the return code describer for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_DESCRIBE_RETURN_CODE 
function os::util::install_describe_return_code() {
	export OS_DESCRIBE_RETURN_CODE="true"
	os::util::trap::init_err
}
