#!/bin/bash

# This script builds the dind images so they can be baked into the ami
# to reduce minimize the potential for dnf flakes in ci.
#
# Reference: https://github.com/openshift/origin/issues/11452

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::image "${OS_ROOT}/images/dind"        openshift/dind
os::build::image "${OS_ROOT}/images/dind/node"   openshift/dind-node
os::build::image "${OS_ROOT}/images/dind/master" openshift/dind-master

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
