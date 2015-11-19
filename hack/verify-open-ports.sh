#!/bin/bash

# Script to create latest swagger spec.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

os::log::install_errexit

# Open port scanning
echo "[INFO] Checking open ports ('sudo openshift start' should already be running)"

# 53 (DNS)
# 4001,7001 (etcd)
# 8443 (master, api, web)
# 10250 (kubelet)
expected_ports=(53 4001 7001 8443 10250 single-high-port-for-kubelet-api-proxy)

open_ports=($(pgrep -f 'openshift|_output/local' | \
  while read pid; do
    sudo netstat -tulpn 2>/dev/null | grep $pid | \
    while read listening; do
      echo "$listening" | awk '{print $4}' | awk -F: '{print $NF}'
    done
  done | sort -n -u))

if [[ "${#expected_ports[@]}" == "${#open_ports[@]}" ]]; then
    for (( i=0; i<${#expected_ports[@]}-1; i++ )); do
        if [[ "${expected_ports[i]}" != "${open_ports[i]}" ]]; then
            echo "Expected: ${expected_ports[@]}"
            echo "Open:     ${open_ports[@]}"
            exit 1
        fi
    done
    echo "Found expected ports open (${open_ports[@]})"
    exit 0
else
    echo "Expected: ${expected_ports[@]}"
    echo "Open:     ${open_ports[@]}"
    exit 1
fi
