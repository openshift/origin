#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

sed -i s/^Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers

fixup=/data/src/github.com/openshift/origin/hack/vm-provision-fixup.sh
if [[ -x $fixup ]]; then
  $fixup
fi

if [ -f /usr/bin/generate_openshift_service ]
then
  sudo /usr/bin/generate_openshift_service
fi

