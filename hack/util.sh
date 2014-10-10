#!/bin/bash

# Provides simple utility functions

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
