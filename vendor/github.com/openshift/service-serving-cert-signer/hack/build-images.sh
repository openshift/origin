#!/bin/bash

# This script builds all images locally except the base and release images,
# which are handled by hack/build-base-images.sh.

# NOTE:  you only need to run this script if your code changes are part of
# any images OpenShift runs internally such as origin-sti-builder, origin-docker-builder,
# origin-deployer, etc.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::util::ensure::gopath_binary_exists imagebuilder
# image builds require RPMs to have been built
os::build::release::check_for_rpms

# we need to mount RPMs into the container builds for installation
cat <<END > "${OS_OUTPUT_RPMPATH}/_local.repo"
[origin-local-release]
name = OpenShift Origin Release from Local Source
baseurl = file:///srv/origin-local-release/
gpgcheck = 0
enabled = 0
END
OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_ARGS:-} -mount ${OS_OUTPUT_RPMPATH}/:/srv/origin-local-release/ -mount ${OS_OUTPUT_RPMPATH}/_local.repo:/etc/yum.repos.d/origin-local-release.repo"

os::build::images