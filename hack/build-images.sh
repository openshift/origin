#!/bin/bash

# This script builds all images locally except the base and release images,
# which are handled by hack/build-base-images.sh.

# NOTE:  you only need to run this script if your code changes are part of
# any images OpenShift runs internally such as origin-sti-builder, origin-docker-builder,
# origin-deployer, etc.
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

if [[ "${OS_RELEASE:-}" == "n" ]]; then
	# Use local binaries
	imagedir="${OS_OUTPUT_BINPATH}/linux/amd64"
	# identical to build-cross.sh
	os::build::os_version_vars
	if [[ -z "${OS_RELEASE_LOCAL:-}" ]]; then
		OS_RELEASE_COMMIT="${OS_GIT_VERSION//+/-}"
		platform="$(os::build::host_platform)"
		OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]:-${platform}}")
		OS_IMAGE_COMPILE_TARGETS=("${OS_IMAGE_COMPILE_TARGETS[@]:-${OS_IMAGE_COMPILE_TARGETS_LINUX[@]}}")
		OS_SCRATCH_IMAGE_COMPILE_TARGETS=("${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]:-${OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX[@]}}")
		readonly OS_GOFLAGS_TAGS="include_gcs include_oss"

		echo "Building images from source ${OS_RELEASE_COMMIT}:"
		echo
		os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS[@]-}" "${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]-}"
		os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"
		echo
	fi
	# Link or copy primary binaries to the appropriate locations.
	os::util::ln_or_cp "${imagedir}/openshift"       images/origin/bin
	os::util::ln_or_cp "${imagedir}/pod"             images/pod/bin
	os::util::ln_or_cp "${imagedir}/hello-openshift" examples/hello-openshift/bin
	os::util::ln_or_cp "${imagedir}/gitserver"       examples/gitserver/bin
	os::util::ln_or_cp "${imagedir}/dockerregistry"  images/dockerregistry/bin
	# Copy SDN scripts into images/node
	source "${OS_ROOT}/contrib/node/install-sdn.sh"
	os::provision::install-sdn "${OS_ROOT}" "${imagedir}" "${OS_ROOT}/images/node"
	mkdir -p images/node/conf/
	cp -pf "${OS_ROOT}/contrib/systemd/openshift-sdn-ovs.conf" images/node/conf/
else
	os::util::ensure::gopath_binary_exists imagebuilder
	# image builds require RPMs to have been built
	os::build::release::check_for_rpms
	# OS_RELEASE_COMMIT is required by image-build
	os::build::detect_local_release_tars $(os::build::host_platform_friendly)
	# we need to mount RPMs into the container builds for installation
	OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_ARGS:-} -mount ${OS_LOCAL_RPMPATH}/:/srv/origin-local-release/"
fi

# image-build is wrapped to allow output to be captured
function image-build() {
	local tag=$1
	local dir=$2
	local dest="${tag}"
	local extra=
	if [[ ! "${tag}" == *":"* ]]; then
		dest="${tag}:latest"
		# tag to release commit unless we specified a hardcoded tag
		extra="${tag}:${OS_RELEASE_COMMIT}"
	fi
	local dockerfile=
	if [[ "${OS_RELEASE:-}" == "n" && -f "${dir}/Dockerfile.dev" ]]; then
		dockerfile="${dir}/Dockerfile.dev"
	fi

	local STARTTIME
	local ENDTIME
	STARTTIME="$(date +%s)"

	# build the image
	if ! os::build::image "${dir}" "${dest}" "${dockerfile}" "${extra}"; then
		os::log::warning "Retrying build once"
		if ! os::build::image "${dir}" "${dest}" "${dockerfile}" "${extra}"; then
			return 1
		fi
	fi

	# ensure the temporary contents are cleaned up
	git clean -fdx "${dir}"

	ENDTIME="$(date +%s)"
	echo "Finished in $(($ENDTIME - $STARTTIME)) seconds"
}

# builds an image and tags it two ways - with latest, and with the release tag
function image() {
	local tag=$1
	local dir=$2
	local out
	mkdir -p "${BASETMPDIR}"
	out="$( mktemp "${BASETMPDIR}/imagelogs.XXXXX" )"
	if ! image-build "${tag}" "${dir}" > "${out}" 2>&1; then
		sed -e "s|^|$1: |" "${out}" 1>&2
		os::log::error "Failed to build $1"
		return 1
	fi
	sed -e "s|^|$1: |" "${out}"
	return 0
}

# Link or copy image binaries to the appropriate locations.
os::util::ln_or_cp "${OS_OUTPUT_BINPATH}/linux/amd64/hello-openshift" examples/hello-openshift/bin
os::util::ln_or_cp "${OS_OUTPUT_BINPATH}/linux/amd64/gitserver"       examples/gitserver/bin

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

# images that depend on "${tag_prefix}-source"
image "${tag_prefix}-pod"                   images/pod
# images that depend on "${tag_prefix}-base"
image "${tag_prefix}"                       images/origin
image "${tag_prefix}-haproxy-router"        images/router/haproxy
image "${tag_prefix}-keepalived-ipfailover" images/ipfailover/keepalived
image "${tag_prefix}-docker-registry"       images/dockerregistry
image "${tag_prefix}-egress-router"         images/egress/router
# images that depend on "${tag_prefix}
image "${tag_prefix}-gitserver"             examples/gitserver
image "${tag_prefix}-deployer"              images/deployer
image "${tag_prefix}-recycler"              images/recycler
image "${tag_prefix}-docker-builder"        images/builder/docker/docker-builder
image "${tag_prefix}-sti-builder"           images/builder/docker/sti-builder
image "${tag_prefix}-f5-router"             images/router/f5
image "openshift/node"                      images/node
# images that depend on "openshift/node"
image "openshift/openvswitch"               images/openvswitch

# extra images (not part of infrastructure)
image "openshift/hello-openshift"           examples/hello-openshift

echo

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
