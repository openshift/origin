#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ENTRY="$2\t$1"
if ! grep -qP "${ENTRY}" /etc/hosts; then
  # The ip + hostname combination are not present
  if grep -qP "\t$1$" /etc/hosts; then
    # The hostname is present with a different ip
    /usr/local/bin/openshift-remove-from-hosts.sh "$1"
  fi
  echo -e "${ENTRY}" >> /etc/hosts
fi
