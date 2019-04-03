#!/bin/sh

version="$( uname -r )"


if [[ $version != *"el8"* ]]; then
        exec chroot RHEL7/ /usr/local/bin/openshift-sdn
else
        exec /usr/local/bin/openshift-sdn
fi
