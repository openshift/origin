#!/bin/bash

# Provides simple utility functions

TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))

# wait_for_command executes a command and waits for it to
# complete or times out after max_wait.
#
# $1 - The command to execute (e.g. curl -fs http://redhat.com)
# $2 - Optional maximum time to wait before giving up (Default: 10s)
# $3 - Optional alternate command to determine if the wait should
#      exit before the max_wait
function wait_for_command {
  STARTTIME=$(date +%s)
  cmd=$1
  msg="Waiting for command to finish: '${cmd}'..."
  max_wait=${2:-10*TIME_SEC}
  fail=${3:-""}
  wait=0.2

  echo "[INFO] $msg"
  expire=$(($(time_now) + $max_wait))
  set +e
  while [[ $(time_now) -lt $expire ]]; do
    eval $cmd
    if [ $? -eq 0 ]; then
      set -e
      ENDTIME=$(date +%s)
      echo "[INFO] Success running command: '$cmd' after $(($ENDTIME - $STARTTIME)) seconds"
      return 0
    fi
    #check a failure condition where the success
    #command may never be evaulated before timing
    #out
    if [[ ! -z $fail ]]; then
      eval $fail
      if [ $? -eq 0 ]; then
        set -e
        echo "[FAIL] Returning early. Command Failed '$cmd'"
        return 1
      fi
    fi
    sleep $wait
  done
  echo "[ ERR] Gave up waiting for: '$cmd'"
  set -e
  return 1
}

# wait_for_url_timed attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional maximum time to wait before giving up (Default: 10s)
function wait_for_url_timed {
  STARTTIME=$(date +%s)
  url=$1
  prefix=${2:-}
  max_wait=${3:-10*TIME_SEC}
  wait=0.2
  expire=$(($(time_now) + $max_wait))
  set +e
  while [[ $(time_now) -lt $expire ]]; do
    out=$(curl --max-time 2 -fs $url 2>/dev/null)
    if [ $? -eq 0 ]; then
      set -e
      echo ${prefix}${out}
      ENDTIME=$(date +%s)
      echo "[INFO] Success accessing '$url' after $(($ENDTIME - $STARTTIME)) seconds"
      return 0
    fi
    sleep $wait
  done
  echo "ERROR: gave up waiting for $url"
  set -e
  return 1
}

# wait_for_file returns 0 if a file exists, 1 if it does not exist
#
# $1 - The file to check for existence
# $2 - Optional time to sleep between attempts (Default: 0.2s)
# $3 - Optional number of attemps to make (Default: 10)
function wait_for_file {
  file=$1
  wait=${2:-0.2}
  times=${3:-10}
  for i in $(seq 1 $times); do
    if [ -f "${file}" ]; then
      return 0
    fi
    sleep $wait
  done
  echo "ERROR: gave up waiting for file ${file}"
  return 1
}

# wait_for_url attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional time to sleep between attempts (Default: 0.2s)
# $4 - Optional number of attemps to make (Default: 10)
function wait_for_url {
  url=$1
  prefix=${2:-}
  wait=${3:-0.2}
  times=${4:-10}

  set_curl_args $wait $times

  set +e
  cmd="env -i CURL_CA_BUNDLE=${CURL_CA_BUNDLE:-} $(which curl) ${clientcert_args} -fs ${url}"
  #echo "run: ${cmd}"
  for i in $(seq 1 $times); do
    out=$(${cmd})
    if [ $? -eq 0 ]; then
      set -e
      echo "${prefix}${out}"
      return 0
    fi
    sleep $wait
  done
  echo "ERROR: gave up waiting for ${url}"
  echo $(${cmd})
  set -e
  return 1
}

# set_curl_args tries to export CURL_ARGS for a program to use.
# will do a wait for the files to exist when using curl with
# SecureTransport (because we must convert the keys to a different
# form).
#
# $1 - Optional time to sleep between attempts (Default: 0.2s)
# $2 - Optional number of attemps to make (Default: 10)
function set_curl_args {
  wait=${1:-0.2}
  times=${2:-10}

  CURL_CERT=${CURL_CERT:-}
  CURL_KEY=${CURL_KEY:-}
  clientcert_args="${CURL_EXTRA:-} "

  if [ -n "${CURL_CERT}" ]; then
   if [ -n "${CURL_KEY}" ]; then
     if [[ `curl -V` == *"SecureTransport"* ]]; then
       # Convert to a p12 cert for SecureTransport
       export CURL_CERT_DIR=$(dirname "${CURL_CERT}")
       export CURL_CERT_P12=${CURL_CERT_P12:-${CURL_CERT_DIR}/cert.p12}
       export CURL_CERT_P12_PASSWORD=${CURL_CERT_P12_PASSWORD:-password}
       if [ ! -f "${CURL_CERT_P12}" ]; then
         wait_for_file "${CURL_CERT}" $wait $times
         wait_for_file "${CURL_KEY}" $wait $times
         openssl pkcs12 -export -inkey "${CURL_KEY}" -in "${CURL_CERT}" -out "${CURL_CERT_P12}" -password "pass:${CURL_CERT_P12_PASSWORD}"
       fi
       clientcert_args="--cert ${CURL_CERT_P12}:${CURL_CERT_P12_PASSWORD} ${CURL_EXTRA:-}"
     else
       clientcert_args="--cert ${CURL_CERT} --key ${CURL_KEY} ${CURL_EXTRA:-}"
     fi
   fi
  fi
  export CURL_ARGS="${clientcert_args}"
}

# Search for a regular expression in a HTTP response.
#
# $1 - a valid URL (e.g.: http://127.0.0.1:8080)
# $2 - a regular expression or text
function validate_response {
  url=$1
  expected_response=$2
  wait=${3:-0.2}
  times=${4:-10}

  set +e
  for i in $(seq 1 $times); do
    response=`curl $url`
    echo $response | grep -q "$expected_response"
    if [ $? -eq 0 ]; then
      echo "[INFO] Response is valid."
      set -e
      return 0
    fi
    sleep $wait
  done

  echo "[INFO] Response is invalid: $response"
  set -e
  return 1
}


# start_etcd starts an etcd server
# $1 - Optional host (Default: 127.0.0.1)
# $2 - Optional port (Default: 4001)
function start_etcd {
  [ ! -z "${ETCD_STARTED-}" ] && return

  host=${ETCD_HOST:-127.0.0.1}
  port=${ETCD_PORT:-4001}

  set +e

  if [ "$(which etcd)" == "" ]; then
    echo "etcd must be in your PATH"
    exit 1
  fi

  running_etcd=$(ps -ef | grep etcd | grep -c name)
  if [ "$running_etcd" != "0" ]; then
    echo "etcd appears to already be running on this machine, please kill and restart the test."
    exit 1
  fi

  # Stop on any failures
  set -e

  # get etcd version
  etcd_version=$(etcd --version | awk '{print $3}')
  initial_cluster=""
  if [[ "${etcd_version}" =~ ^2 ]]; then
    initial_cluster="--initial-cluster test=http://localhost:2380,test=http://localhost:7001"
  fi

  # Start etcd
  export ETCD_DIR=$(mktemp -d -t test-etcd.XXXXXX)
  etcd -name test -data-dir ${ETCD_DIR} -bind-addr ${host}:${port} ${initial_cluster} >/dev/null 2>/dev/null &
  export ETCD_PID=$!

  wait_for_url "http://${host}:${port}/version" "etcd: " 0.25 80
  curl -X PUT  "http://${host}:${port}/v2/keys/_test"
  echo
}

# stop_openshift_server utility function to terminate an
# all-in-one running instance of OpenShift
function stop_openshift_server()
{
    set +e
    set +u
    if [ -n $OS_PID ] ; then
      echo "[INFO] Found running OpenShift Server instance"
      sudo kill -INT $OS_PID 1>&2 2>/dev/null
      unset OS_PID
      echo "[INFO] Terminated OpenShift Server"
    fi
    set -u
    set -e
}

# time_now return the time since the epoch in millis
function time_now()
{
  echo $(date +%s000)
}

# Handler for when we exit automatically on an error.
# Borrowed from https://gist.github.com/ahendrix/7030300
os::log::errexit() {
  local err="${PIPESTATUS[@]}"

  # If the shell we are in doesn't have errexit set (common in subshells) then
  # don't dump stacks.
  set +o | grep -qe "-o errexit" || return

  set +o xtrace
  local code="${1:-1}"
  os::log::error_exit "'${BASH_COMMAND}' exited with status $err" "${1:-1}" 1
}

os::log::install_errexit() {
  # trap ERR to provide an error handler whenever a command exits nonzero  this
  # is a more verbose version of set -o errexit
  trap 'os::log::errexit' ERR

  # setting errtrace allows our ERR trap handler to be propagated to functions,
  # expansions and subshells
  set -o errtrace
}

# Print out the stack trace
#
# Args:
#   $1 The number of stack frames to skip when printing.
os::log::stack() {
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
#   $1 Message to log with the error
#   $2 The error code to return
#   $3 The number of stack frames to skip when printing.
os::log::error_exit() {
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

  os::log::stack $stack_skip

  echo "Exiting with status ${code}" >&2
  exit "${code}"
}

# Log an error but keep going.  Don't dump the stack or exit.
os::log::error() {
  echo "!!! ${1-}" >&2
  shift
  for message; do
    echo "    $message" >&2
  done
}

# Print an usage message to stderr.  The arguments are printed directly.
os::log::usage() {
  echo >&2
  local message
  for message; do
    echo "$message" >&2
  done
  echo >&2
}

os::log::usage_from_stdin() {
  local messages=()
  while read -r line; do
    messages+=$line
  done

  os::log::usage "${messages[@]}"
}

# Print out some info that isn't a top level status line
os::log::info() {
  for message; do
    echo "$message"
  done
}

os::log::info_from_stdin() {
  local messages=()
  while read -r line; do
    messages+=$line
  done

  os::log::info "${messages[@]}"
}

# Print a status line.  Formatted to show up in a stream of output.
os::log::status() {
  echo "+++ $1"
  shift
  for message; do
    echo "    $message"
  done
}
