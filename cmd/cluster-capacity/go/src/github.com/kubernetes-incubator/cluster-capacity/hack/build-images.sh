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
		OS_SCRATCH_IMAGE_COMPILE_TARGETS=("${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]:-}")
		readonly OS_GOFLAGS_TAGS="include_gcs include_oss"

		echo "Building images from source ${OS_RELEASE_COMMIT}:"
		echo
		os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS[@]-}"
		os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"
		echo
	fi
else
	# Get the latest Linux release
	if [[ ! -d _output/local/releases ]]; then
		echo "No release has been built. Run hack/build-release.sh"
		exit 1
	fi

	# Extract the release archives to a staging area.
	os::build::detect_local_release_tars "linux-64bit"

	echo "Building images from release tars for commit ${OS_RELEASE_COMMIT}:"
	echo " primary: $(basename ${OS_PRIMARY_RELEASE_TAR})"
	echo " image:   $(basename ${OS_IMAGE_RELEASE_TAR})"

	imagedir="${OS_OUTPUT}/images"
	rm -rf ${imagedir}
	mkdir -p ${imagedir}
	os::build::extract_tar "${OS_PRIMARY_RELEASE_TAR}" "${imagedir}"
	os::build::extract_tar "${OS_IMAGE_RELEASE_TAR}" "${imagedir}"
fi

# Create link to file if the FS supports hardlinks, otherwise copy the file
function ln_or_cp {
	local src_file=$1
	local dst_dir=$2
	if os::build::is_hardlink_supported "${dst_dir}" ; then
		ln -f "${src_file}" "${dst_dir}"
	else
		cp -pf "${src_file}" "${dst_dir}"
	fi
}


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

	local STARTTIME
	local ENDTIME
	STARTTIME="$(date +%s)"

	# build the image
	if ! os::build::image "${dir}" "${dest}" "" "${extra}"; then
		os::log::warning "Retrying build once"
		if ! os::build::image "${dir}" "${dest}" "" "${extra}"; then
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

# Link or copy primary binaries to the appropriate locations.
ln_or_cp "${imagedir}/hypercc"         images/cluster-capacity/bin

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

image "${tag_prefix}-cluster-capacity"      images/cluster-capacity

echo

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
