#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh

PUBLIC_KEY="/data/id_rsa.pub"

if os::util::is-master; then
  # Generate the keypair
  ssh-keygen -N '' -q -f /root/.ssh/id_rsa
  cp /root/.ssh/id_rsa.pub "${PUBLIC_KEY}"
else
  # Wait for the master to generate the keypair
  CONDITION="test -f ${PUBLIC_KEY}"
  os::util::wait-for-condition "public key to be generated" "${CONDITION}"
fi

mkdir -p /root/.ssh
chmod 700 /root/.ssh
cp "${PUBLIC_KEY}" /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
