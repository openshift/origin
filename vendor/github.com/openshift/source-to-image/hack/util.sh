#!/bin/bash

# Provides simple utility functions

# Handler for when we exit automatically on an error.
# Borrowed from https://gist.github.com/ahendrix/7030300
s2i::log::errexit() {
  local err="${PIPESTATUS[@]}"

  # If the shell we are in doesn't have errexit set (common in subshells) then
  # don't dump stacks.
  set +o | grep -qe "-o errexit" || return

  set +o xtrace
  local code="${1:-1}"
  s2i::log::error_exit "'${BASH_COMMAND}' exited with status $err" "${1:-1}" 1
}

s2i::log::install_errexit() {
  # trap ERR to provide an error handler whenever a command exits nonzero this
  # is a more verbose version of set -o errexit
  trap 's2i::log::errexit' ERR

  # setting errtrace allows our ERR trap handler to be propagated to functions,
  # expansions and subshells
  set -o errtrace
}

# Print out the stack trace
#
# Args:
#  $1 The number of stack frames to skip when printing.
s2i::log::stack() {
  local stack_skip=${1:-0}
  stack_skip=$((stack_skip + 1))
  if [[ ${#FUNCNAME[@]} -gt $stack_skip ]]; then
  echo "Call stack:" >&2
  local i
  for ((i=1 ; i <= ${#FUNCNAME[@]} - $stack_skip ; i++))
  do
    local frame_no=$((i - 1 + stack_skip))
    local source_file=${BASH_SOURCE[$frame_no]}
    local source_lineno=${BASH_LINENO[$((frame_no - 1))]}
    local funcname=${FUNCNAME[$frame_no]}
    echo "  $i: ${source_file}:${source_lineno} ${funcname}(...)" >&2
  done
  fi
}

# Log an error and exit.
# Args:
#  $1 Message to log with the error
#  $2 The error code to return
#  $3 The number of stack frames to skip when printing.
s2i::log::error_exit() {
  local message="${1:-}"
  local code="${2:-1}"
  local stack_skip="${3:-0}"
  stack_skip=$((stack_skip + 1))

  local source_file=${BASH_SOURCE[$stack_skip]}
  local source_line=${BASH_LINENO[$((stack_skip - 1))]}
  echo "!!! Error in ${source_file}:${source_line}" >&2
  [[ -z ${1-} ]] || {
  echo "  ${1}" >&2
  }

  s2i::log::stack $stack_skip

  echo "Exiting with status ${code}" >&2
  exit "${code}"
}

s2i::util::sed() {
  if [[ "$(go env GOHOSTOS)" == "darwin" ]]; then
    sed -i '' $@
  else
    sed -i'' $@
  fi
}

s2i::util::find_files() {
  find . -type d -name vendor -prune -o -name '*.go' -print
}
