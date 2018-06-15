#!/bin/bash

# This script builds the dind images so they can be baked into the ami
# to reduce minimize the potential for dnf flakes in ci.
#
# Reference: https://github.com/openshift/origin/issues/11452

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
	return_code=$?
	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

os::build::image openshift/dind        "${OS_ROOT}/images/dind"
os::build::image openshift/dind-node   "${OS_ROOT}/images/dind/node"
os::build::image openshift/dind-master "${OS_ROOT}/images/dind/master"
