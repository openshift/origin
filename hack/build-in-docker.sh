#!/bin/bash

# This script build the sources in openshift/origin-release image using
# the Fedora environment and Go compiler.

set -o errexit
set -o nounset
set -o pipefail

origin_path="src/github.com/openshift/origin"

# TODO: Remove this check and fix the docker command instead after the
#       following PR is merged: https://github.com/docker/docker/pull/5910
#       should be done in docker 1.6.
if [ -d /sys/fs/selinux ]; then
    if ! ls --context "${GOPATH}/${origin_path}" | grep --quiet svirt_sandbox_file_t; then
        echo "$(tput setaf 1)Warning: SELinux labels are not set correctly; run chcon -Rt svirt_sandbox_file_t ${GOPATH}/${origin_path}$(tput sgr0)"
        exit 1
    fi
fi

docker run --rm -v "${GOPATH}/${origin_path}:/go/${origin_path}" \
  openshift/origin-release /usr/bin/openshift-origin-build.sh "$@"
