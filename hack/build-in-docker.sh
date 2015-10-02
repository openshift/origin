#!/bin/bash

# This script build the sources in openshift/origin-release image using
# the Fedora environment and Go compiler.

set -o errexit
set -o nounset
set -o pipefail

function absolute_path() { 
  pushd . > /dev/null
  [ -d "$1" ] && cd "$1" && dirs -l +0
  popd > /dev/null
}

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
origin_path="src/github.com/openshift/origin"

# TODO: Remove this check and fix the docker command instead after the
#       following PR is merged: https://github.com/docker/docker/pull/5910
#       should be done in docker 1.6.
if [ -d /sys/fs/selinux ]; then
    if ! ls --context "$(absolute_path $OS_ROOT)" | grep --quiet svirt_sandbox_file_t; then
        echo "$(tput setaf 1)Warning: SELinux labels are not set correctly; run chcon -Rt svirt_sandbox_file_t $(absolute_path $OS_ROOT)$(tput sgr0)"
        exit 1
    fi
fi

docker run -e "OWNER_GROUP=$UID:$GROUPS" --rm -v "$(absolute_path $OS_ROOT):/go/${origin_path}" openshift/origin-release /usr/bin/openshift-origin-build.sh $@

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
