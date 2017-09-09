#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

if [[ "${OPENSHIFT_CONTAINER_RUNTIME}" = "crio" ]]; then
  systemctl enable crio
  systemctl start crio
fi
