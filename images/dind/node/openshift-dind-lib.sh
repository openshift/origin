#!/bin/bash
#
# This library holds utility functions used by dind deployment and images.  Since
# it is intended to be distributed standalone in dind images, it cannot depend
# on any functions outside of this file.

# os::util::wait-for-condition blocks until the provided condition becomes true
#
# Globals:
#  None
# Arguments:
#  - 1: message indicating what conditions is being waited for (e.g. 'config to be written')
#  - 2: a string representing an eval'able condition.  When eval'd it should not output
#       anything to stdout or stderr.
#  - 3: optional timeout in seconds.  If not provided, defaults to 60s.  If OS_WAIT_FOREVER
#       is provided, wait forever.
# Returns:
#  1 if the condition is not met before the timeout
readonly OS_WAIT_FOREVER=-1
function os::util::wait-for-condition() {
  local msg=$1
  # condition should be a string that can be eval'd.  When eval'd, it
  # should not output anything to stderr or stdout.
  local condition=$2
  local timeout=${3:-60}

  local start_msg="Waiting for ${msg}"
  local error_msg="[ERROR] Timeout waiting for ${msg}"

  local counter=0
  while ! ${condition}; do
    if [[ "${counter}" = "0" ]]; then
      echo "${start_msg}"
    fi

    if [[ "${counter}" -lt "${timeout}" ||
            "${timeout}" = "${OS_WAIT_FOREVER}" ]]; then
      counter=$((counter + 1))
      if [[ "${timeout}" != "${OS_WAIT_FOREVER}" ]]; then
        echo -n '.'
      fi
      sleep 1
    else
      echo -e "\n${error_msg}"
      return 1
    fi
  done

  if [[ "${counter}" != "0" && "${timeout}" != "${OS_WAIT_FOREVER}" ]]; then
    echo -e '\nDone'
  fi
}
readonly -f os::util::wait-for-condition
