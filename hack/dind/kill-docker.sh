#!/bin/bash

# Ensure that docker is gracefully killed.

set -o errexit
set -o nounset
set -o pipefail

pid_file=/var/run/docker.pid
if [ -f "${pid_file}" ]; then
  pid=$(cat "${pid_file}")
  kill "${pid}"
  echo "Waiting for docker daemon to exit"
  COUNTER=0
  TIMEOUT=60
  while [ -d "/proc/${pid}" ]; do
    if [[ "${COUNTER}" -lt "${TIMEOUT}" ]]; then
      COUNTER=$((COUNTER + 1))
      echo -n '.'
      sleep 1
    else
      echo -e "\nError: Timeout waiting for the docker daemon to exit"
      exit 1
    fi
  done
  echo -e '\nDone'
fi
