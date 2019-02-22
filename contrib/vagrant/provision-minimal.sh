#!/bin/bash

# contrib/vagrant/provision-minimal.sh is referenced by the origin Vagrantfile,
# which in turn is used by the source-to-image Jenkins jobs on ci.openshift.redhat.com
#
# TODO: Remove this once source-to-image uses ci-operator

set -euo pipefail
IFS=$'\n\t'

sed -i s/^Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers

# patch incompatible with fail-over DNS setup
SCRIPT='/etc/NetworkManager/dispatcher.d/fix-slow-dns'
if [[ -f "${SCRIPT}" ]]; then
    echo "Removing ${SCRIPT}..."
    rm "${SCRIPT}"
    sed -i -e '/^options.*$/d' /etc/resolv.conf
fi
unset SCRIPT

if [ -f /usr/bin/generate_openshift_service ]
then
  sudo /usr/bin/generate_openshift_service
fi

