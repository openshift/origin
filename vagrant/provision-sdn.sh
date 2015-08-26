#!/bin/bash
set -ex
source $(dirname $0)/provision-config.sh

os::util::install-sdn "${ORIGIN_ROOT}"

# Only start openvswitch if it has been installed (only minions).
if rpm -qa | grep -q openvswitch; then
  systemctl enable openvswitch
  systemctl start openvswitch
fi

# no need to start openshift-sdn, as it is integrated with openshift binary
