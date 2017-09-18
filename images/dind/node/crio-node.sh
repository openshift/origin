#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

if [[ "${OPENSHIFT_CONTAINER_RUNTIME}" = "crio" ]]; then
  ln -sf /data/crio /usr/bin/
  ln -sf /data/crioctl /usr/bin/
  ln -sf /data/kpod /usr/bin/
  mkdir -p /usr/libexec/crio
  ln -sf /data/conmon /usr/libexec/crio/
  ln -sf /data/pause /usr/libexec/crio/
  ln -sf /data/crio.conf /etc/
  ln -sf /data/seccomp.json /etc/

  systemctl enable crio
  systemctl start crio
fi
