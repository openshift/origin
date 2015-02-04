#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

sed -i s/^Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers

if [ -f /usr/bin/generate_openshift_service ]
then
  sudo /usr/bin/generate_openshift_service
fi