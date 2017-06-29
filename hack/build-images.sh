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
# OS_RELEASE_COMMIT is required by image-build
os::build::archive::detect_local_release_tars $(os::build::host_platform_friendly)

# we need to mount RPMs into the container builds for installation
OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_ARGS:-} -mount ${OS_OUTPUT_RPMPATH}/:/srv/origin-local-release/"

# Create link to file if the FS supports hardlinks, otherwise copy the file
function ln_or_cp {
	local src_file=$1
	local dst_dir=$2
	if os::build::archive::internal::is_hardlink_supported "${dst_dir}" ; then
		ln -f "${src_file}" "${dst_dir}"
	else
		cp -pf "${src_file}" "${dst_dir}"
	fi
}

# Link or copy image binaries to the appropriate locations.
ln_or_cp "${OS_OUTPUT_BINPATH}/linux/amd64/hello-openshift" examples/hello-openshift/bin
ln_or_cp "${OS_OUTPUT_BINPATH}/linux/amd64/gitserver"       examples/gitserver/bin

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

# images that depend on "${tag_prefix}-source"
os::build::image "${tag_prefix}-pod"                   images/pod
os::build::image "${tag_prefix}-cluster-capacity"      images/cluster-capacity
os::build::image "${tag_prefix}-service-catalog"       images/service-catalog

# images that depend on "${tag_prefix}-base"
os::build::image "${tag_prefix}"                       images/origin
os::build::image "${tag_prefix}-haproxy-router"        images/router/haproxy
os::build::image "${tag_prefix}-keepalived-ipfailover" images/ipfailover/keepalived
os::build::image "${tag_prefix}-docker-registry"       images/dockerregistry
os::build::image "${tag_prefix}-egress-router"         images/egress/router
os::build::image "${tag_prefix}-egress-http-proxy"     images/egress/http-proxy
os::build::image "${tag_prefix}-federation"            images/federation
# images that depend on "${tag_prefix}
os::build::image "${tag_prefix}-gitserver"             examples/gitserver
os::build::image "${tag_prefix}-deployer"              images/deployer
os::build::image "${tag_prefix}-recycler"              images/recycler
os::build::image "${tag_prefix}-docker-builder"        images/builder/docker/docker-builder
os::build::image "${tag_prefix}-sti-builder"           images/builder/docker/sti-builder
os::build::image "${tag_prefix}-f5-router"             images/router/f5
os::build::image "openshift/node"                      images/node
# images that depend on "openshift/node"
os::build::image "openshift/openvswitch"               images/openvswitch

# extra images (not part of infrastructure)
os::build::image "openshift/hello-openshift"           examples/hello-openshift