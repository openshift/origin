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

  set +e
  for i in $(seq 1 $times); do
    out=$(curl -fs $url 2>/dev/null)
    if [ $? -eq 0 ]; then
      set -e
      echo ${prefix}${out}
      return 0
    fi
    sleep $wait
  done
  echo "ERROR: gave up waiting for $url"
  curl $url
  set -e
  return 1
}

# start_etcd starts an etcd server
# $1 - Optional host (Default: 127.0.0.1)
# $2 - Optional port (Default: 4001)
function start_etcd {
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

  # Start etcd
  export ETCD_DIR=$(mktemp -d -t test-etcd.XXXXXX)
  etcd -name test -data-dir ${ETCD_DIR} -bind-addr ${host}:${port} >/dev/null 2>/dev/null &
  export ETCD_PID=$!

  wait_for_url "http://127.0.0.1:4001/v2/keys/" "etcd: "
}

# start up openshift server
# $1 - volume dir
# $2 - etcd data dir
# $3 - log dir
function start_openshift_server()
{
  "${openshift}" start --volume-dir="${1}" --etcd-dir="${2}" --loglevel=4 &> "${3}/openshift.log" &
  export OS_PID=$!
}

# stop_openshift_server utility function to terminate an
# all-in-one running instance of OpenShift
function stop_openshift_server()
{
    set +e
    set +u
    if [ -n $OS_PID ] ; then
      echo "[INFO] Found running OpenShift Server instance"
      kill $OS_PID 1>&2 2>/dev/null
      echo "[INFO] Terminated OpenShift Server"
    fi
    set -u
    set -e
}

# time_now return the time since the epoch in millis
function time_now()
{
  echo $(($(date +'%s * 1000 + %-N / 1000000')))
}
