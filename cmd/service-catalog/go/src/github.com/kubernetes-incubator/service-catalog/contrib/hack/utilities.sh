# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# Library of useful utilities.

# Exit with a message and an exit code.
# Arguments:
#   $1 - string with an error message
#   $2 - exit code, defaults to 1
function error_exit() {
  # ${BASH_SOURCE[1]} is the file name of the caller.
  echo "${BASH_SOURCE[1]}: line ${BASH_LINENO[0]}: ${1:-Unknown Error.} (exit ${2:-1})" 1>&2
  exit ${2:-1}
}

# Retries a command with an exponential back-off.
# The back-off base is a constant 3/2
# Options:
#   -n Maximum total attempts (0 for infinite, default 10)
#   -t Maximum time to sleep between retries (default 60)
#   -s Initial time to sleep between retries. Subsequent retries
#      subject to exponential back-off up-to the maximum time.
#      (default 5)
function retry() {
  local OPTIND OPTARG ARG
  local COUNT=10
  local SLEEP=5 MAX_SLEEP=60
  local MUL=3 DIV=2 # Exponent base multiplier and divisor
                    # (Bash doesn't do floats)

  while getopts ":n:s:t:" ARG; do
    case ${ARG} in
      n) COUNT=${OPTARG};;
      s) SLEEP=${OPTARG};;
      t) MAX_SLEEP=${OPTARG};;
      *) echo "Unrecognized argument: -${OPTARG}";;
    esac
  done

  shift $((OPTIND-1))

  # If there is no command, abort early.
  [[ ${#} -le 0 ]] && { echo "No command specified, aborting."; return 1; }

  local N=1 S=${SLEEP}  # S is the current length of sleep.
  while : ; do
    echo "${N}. Executing ${@}"
    "${@}" && { echo "Command succeeded."; return 0; }

    [[ (( COUNT -le 0 || N -lt COUNT )) ]] \
      || { echo "Command '${@}' failed ${N} times, aborting."; return 1; }

    if [[ (( S -lt MAX_SLEEP )) ]] ; then
      # Must always count full exponent due to integer rounding.
      ((S=SLEEP * (MUL ** (N-1)) / (DIV ** (N-1))))
    fi

    ((S=(S < MAX_SLEEP) ? S : MAX_SLEEP))

    echo "Command failed. Will retry in ${S} seconds."
    sleep ${S}

    ((N++))
  done
}

# Will wait until the output of the passed command contains the
# expected substring.
# If the negate flag is passed, waits until output does not contain
# the expected substring.
function wait_for_expected_output() {
  local OPTIND OPTARG ARG
  local negate=''
  local count=10
  local sleep_amount=5
  local max_sleep=60
  local expected=''

  while getopts ":xn:s:t:e:" ARG; do
    case ${ARG} in
      x) negate='YES';;
      n) count=${OPTARG};;
      s) sleep_amount=${OPTARG};;
      t) max_sleep=${OPTARG};;
      e) expected=${OPTARG};;
      *) echo "Unrecognized argument: -${OPTARG}";;
    esac
  done

  shift $((OPTIND-1))

  # If there is no command, abort early.
  [[ ${#} -le 0 ]] && { echo "No command specified, aborting."; return 1; }

  if [[ -n "${negate:-}" ]]; then
    retry -n ${count} -s ${sleep_amount} -t ${max_sleep} \
        output_does_not_contain_substring -e "${expected}" "${@}" > /dev/null \
      && return 0

    echo "Waited unsuccessfully for no occurrence of \"${expected}\" in: \"$("${@}")\""
  else
    retry -n ${count} -s ${sleep_amount} -t ${max_sleep} \
        output_contains_substring -e "${expected}" "${@}" > /dev/null \
      && return 0

    echo "Waited unsuccessfully for occurrence of \"${expected}\" in: \"$("${@}")\""
  fi

  return 1
}

function output_contains_substring() {
  local OPTIND OPTARG ARG
  local expected=''

  while getopts ":e:" ARG; do
    case ${ARG} in
      e) expected=${OPTARG};;
      *) echo "Unrecognized argument: -${OPTARG}";;
    esac
  done

  shift $((OPTIND-1))

  # If there is no command, abort early.
  [[ ${#} -le 0 ]] && { echo "No command specified, aborting."; return 1; }

  [[ "$("${@}")" == *"${expected}"* ]]
}

function output_does_not_contain_substring() {
  output_contains_substring "$@" && return 1
  return 0
}
