#!/bin/sh
#
# This script is expected to be run with:
#
#   $ oc observe csr -a '{.status.conditions[*].type}' -a '{.status.certificate}' -- PATH_TO_SCRIPT
#
# It will approve any CSR that is not approved yet, and delete any CSR that expired more than 60 seconds
# ago.
#

set -o errexit
set -o nounset
set -o pipefail

name=${1}
condition=${2}
certificate=${3}

# auto approve
if [[ -z "${condition}" ]]; then
  oc adm certificate approve "${name}"
  exit 0
fi

# check certificate age
if [[ -n "${certificate}" ]]; then
  text="$( echo "${certificate}" | base64 -D - )"
  if ! echo "${text}" | openssl x509 -checkend -60 > /dev/null; then
    echo "Certificate is expired, deleting"
    oc delete csr "${name}"
  fi
  exit 0
fi
