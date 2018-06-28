#!/bin/bash

# This library holds utility functions for building container images.

# os::build::image builds an image from a directory, to a tag or tags The default
# behavior is to use the imagebuilder binary if it is available on the path with
# fallback to docker build if it is not available.
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
#  - OS_BUILD_IMAGE_NUM_RETRIES
# Arguments:
#  - 1: the directory in which to build
#  - 2: the tag to apply to the image
# Returns:
#  None
function os::build::image() {
	local tag=$1
	local directory=$2
	local extra_tag

	if [[ ! "${tag}" == *":"* ]]; then
		# if no tag was specified in the image name,
		# tag with :latest and the release commit, if
		# available, falling back to the last commit
		# if no release commit is recorded
		local release_commit
		release_commit="${OS_RELEASE_COMMIT-}"
		if [[ -z "${release_commit}" && -f "${OS_OUTPUT_RELEASEPATH}/.commit" ]]; then
			release_commit="$( cat "${OS_OUTPUT_RELEASEPATH}/.commit" )"
		fi
		if [[ -z "${release_commit}" ]]; then
			release_commit="$( git log -1 --pretty=%h )"
		fi
		extra_tag="${tag}:${release_commit}"

		tag="${tag}:latest"
	fi

	local result=1
	local image_build_log
	image_build_log="$( mktemp "${BASETMPDIR}/imagelogs.XXXXX" )"
	for (( i = 0; i < "${OS_BUILD_IMAGE_NUM_RETRIES:-2}"; i++ )); do
		if [[ "${i}" -gt 0 ]]; then
			os::log::internal::prefix_lines "[${tag%:*}]" "$( cat "${image_build_log}" )"
			os::log::warning "Retrying image build for ${tag}, attempt ${i}..."
		fi

		if os::build::image::internal::generic "${tag}" "${directory}" "${extra_tag:-}" >"${image_build_log}" 2>&1; then
			result=0
			break
		fi
	done

	os::log::internal::prefix_lines "[${tag%:*}]" "$( cat "${image_build_log}" )"
	return "${result}"
}
readonly -f os::build::image

# os::build::image::internal::generic builds a container image using either imagebuilder
# or docker, defaulting to imagebuilder if present
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
# Arguments:
#  - 1: the directory in which to build
#  - 2: the tag to apply to the image
#  - 3: optionally, extra tags to add
# Returns:
#  None
function os::build::image::internal::generic() {
	local directory=$2

	local result=1
	if os::util::find::system_binary 'imagebuilder' >/dev/null; then
		if os::build::image::internal::imagebuilder "$@"; then
			result=0
		fi
	else
		os::log::warning "Unable to locate 'imagebuilder' on PATH, falling back to Docker build"
		if os::build::image::internal::docker "$@"; then
			result=0
		fi
	fi

	# ensure the temporary contents are cleaned up
	git clean -fdx "${directory}"
	return "${result}"
}
readonly -f os::build::image::internal::generic

# os::build::image::internal::imagebuilder builds a container image using imagebuilder
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
# Arguments:
#  - 1: the directory in which to build
#  - 2: the tag to apply to the image
#  - 3: optionally, extra tags to add
# Returns:
#  None
function os::build::image::internal::imagebuilder() {
	local tag=$1
	local directory=$2
	local extra_tag="${3-}"
	local options=()

	if [[ -n "${OS_BUILD_IMAGE_ARGS:-}" ]]; then
		options=( ${OS_BUILD_IMAGE_ARGS} )
	fi

	if [[ -n "${extra_tag}" ]]; then
		options+=( -t "${extra_tag}" )
	fi

	imagebuilder "${options[@]:-}" -t "${tag}" "${directory}"
}
readonly -f os::build::image::internal::imagebuilder

# os::build::image::internal::docker builds a container image using docker
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
# Arguments:
#  - 1: the directory in which to build
#  - 2: the tag to apply to the image
#  - 3: optionally, extra tags to add
# Returns:
#  None
function os::build::image::internal::docker() {
	local tag=$1
	local directory=$2
	local extra_tag="${3-}"
	local options=()

	if ! docker build ${OS_BUILD_IMAGE_ARGS:-} -t "${tag}" "${directory}"; then
		return 1
	fi

	if [[ -n "${extra_tag}" ]]; then
		docker tag "${tag}" "${extra_tag}"
	fi
}
readonly -f os::build::image::internal::docker
