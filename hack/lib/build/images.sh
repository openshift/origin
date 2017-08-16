#!/bin/bash

# This library holds utility functions for building container images.

# os::build::cross_images builds an image per supported architecture from a directory,
# to a tag or tags using the imagebuilder binary.
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
#  - OS_BUILD_IMAGE_NUM_RETRIES
# Arguments:
#  - 1: the base name to apply to the images
#  - 2: the directory in which to build
#  - rest: optionally, arches to build followed by tags to apply to the generated names
# Returns:
#  None
function os::build::cross_images() {
	local image_base_name=$1
	local directory=$2
	shift 2

	local host_arch="$(os::build::go_arch)"

	# process arches params
	local -a arches
	while (( "$#" )); do
		# if the next parameter does not match know arches we assume
		# that we are ready to process tags instead
		local arch="${1}"
		[[ "${OS_BUILD_ENV_ARCHES[@]}" =~ "${arch}" ]] || break

		arches+=("$1")
		shift
	done

	# if no arches specified use the default set
	if [[ -z ${arches:-} ]]; then
		os::log::error "At least one arch needs to be specified, supported arches: ${OS_BUILD_ENV_ARCHES[@]}"
		exit 1
	fi

	if [[ "${host_arch}" != "amd64" && ("${arches[0]}" != "${host_arch}" || "${#arches[@]}" > 1) ]]; then
		os::log::error "cross building images is only supported for amd64 systems"
		exit 1
	fi

	# process tags params
	local -a tags_to_apply
	while (( "$#" )); do
		tags_to_apply+=("$1")
		shift
	done

	# if no tags are specified use latest and the commit id
	if [[ -z ${tags_to_apply:-} ]]; then
		tags_to_apply=("latest" "$(os::build::image::internal::release_commit)")
	fi

	local orig_build_image_args=${OS_BUILD_IMAGE_ARGS:-}

	# cross builds are only supported with imagebuilder
	os::util::ensure::gopath_binary_exists imagebuilder


	for arch in ${arches[@]}; do
		local sys_arch=$(os::build::sys_arch $arch)
		OS_BUILD_IMAGE_ARGS=""
		if [[ "${arch}" != "${host_arch}" ]]; then
			# remove all previously registered binfmt_misc entries and
			# register qemu-*-static for all supported processors except the current one
			docker run --rm --privileged multiarch/qemu-user-static:register --reset
			local qemu_binary=qemu-${sys_arch}-static
			os::util::ensure::system_binary_exists $qemu_binary
			local qemu_binary_path=$(os::util::find::system_binary $qemu_binary)
			OS_BUILD_IMAGE_ARGS+="-mount ${qemu_binary_path}:${qemu_binary_path}"
		fi

		# If there is an architecture specific Dockerfile to use, use it
		local dockerfile_path="${directory}/Dockerfile"
		if [[ -f "${dockerfile_path}.${sys_arch}" ]]; then
			dockerfile_path+=".${sys_arch}"
		elif [[ "${arch}" != "${OS_BUILD_ENV_ARCHES[0]}" && -f "${dockerfile_path}.altarch" ]]; then
			dockerfile_path+=".altarch"
		fi

		OS_BUILD_IMAGE_ARGS+=" -f ${dockerfile_path} --from $(os::build::image::internal::get_arch_specific_from ${dockerfile_path} ${arch}) ${orig_build_image_args}"
		local image_tags=()
		for tag in ${tags_to_apply[@]}; do
			image_tags+=("${image_base_name}-${sys_arch}:${tag}")

			# If the build is for the primary arch, add a tag without the architecture
			if [[ "${arch}" == "${OS_BUILD_ENV_ARCHES[0]}" ]]; then
				image_tags+=("${image_base_name}:${tag}")
			fi
		done

		os::build::image "${image_tags[0]}" "${directory}" "${image_tags[@]:1}" || return 1
	done

	OS_BUILD_IMAGE_ARGS=${orig_build_image_args}
}
readonly -f os::build::cross_images

# os::build::image builds an image from a directory, to a tag or tags The default
# behavior is to use the imagebuilder binary if it is available on the path with
# fallback to docker build if it is not available.
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
#  - OS_BUILD_IMAGE_NUM_RETRIES
# Arguments:
#  - 1: the tag to apply to the image
#  - 2: the directory in which to build
#  - rest: optionally, extra tags to add
# Returns:
#  None
function os::build::image() {
	local tag=$1
	local directory=$2
	local extra_tags=("${@:3}")

	if [[ ! "${tag}" == *":"* ]]; then
		# if no tag was specified in the image name,
		# tag with :latest and the release commit, if
		# available, falling back to the last commit
		# if no release commit is recorded
		local release_commit
		release_commit=$(os::build::image::internal::release_commit)
		extra_tags+=("${tag}:${release_commit}")

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

		if os::build::image::internal::generic "${tag}" "${directory}" "${extra_tags[@]:-}" >"${image_build_log}" 2>&1; then
			result=0
			break
		fi
	done

	os::log::internal::prefix_lines "[${tag%:*}]" "$( cat "${image_build_log}" )"
	return "${result}"
}
readonly -f os::build::image

function os::build::image::internal::get_arch_specific_from() {
	dockerfile_path="${1}"
	arch="${2}"
	from_image=$(grep -e "^FROM" "${dockerfile_path}" | awk '{print $2}')
	case "${from_image}" in
		centos:7)
			# replace centos image with arch specific centos image
			echo "$(os::util::centos_image ${arch})"
			;;
		openshift/origin-*)
			# append arch to origin images
			echo "${from_image}-$(os::build::sys_arch $arch)"
			;;
		*)
			# the image as is
			echo "${from_image}"
			;;
	esac
}
readonly -f os::build::image::internal::get_arch_specific_from

function os::build::image::internal::release_commit() {
	echo "${OS_RELEASE_COMMIT:-"$( git log -1 --pretty=%h )"}"
}
readonly -f os::build::image::internal::release_commit

# os::build::image::internal::generic builds a container image using either imagebuilder
# or docker, defaulting to imagebuilder if present
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
# Arguments:
#  - 1: the directory in which to build
#  - 2: the tag to apply to the image
#  - 3: optionally, extra tags to add (space separated)
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
#  - 3: optionally, extra tags to add (space separated)
# Returns:
#  None
function os::build::image::internal::imagebuilder() {
	local tag=$1
	local directory=$2
	local extra_tags=("${@:3}")
	local options=()

	if [[ -n "${OS_BUILD_IMAGE_ARGS:-}" ]]; then
		options=( ${OS_BUILD_IMAGE_ARGS} )
	fi

	for extra_tag in ${extra_tags[@]}; do
		options+=( -t "${extra_tag}" )
	done

	imagebuilder "${options[@]:-}" -t "${tag}" "${directory}" || return 1
}
readonly -f os::build::image::internal::imagebuilder

# os::build::image::internal::docker builds a container image using docker
#
# Globals:
#  - OS_BUILD_IMAGE_ARGS
# Arguments:
#  - 1: the directory in which to build
#  - 2: the tag to apply to the image
#  - 3: optionally, extra tags to add (space separated)
# Returns:
#  None
function os::build::image::internal::docker() {
	local tag=$1
	local directory=$2
	local extra_tags=("${@:3}")
	local options=()

	if ! docker build ${OS_BUILD_IMAGE_ARGS:-} -t "${tag}" "${directory}"; then
		return "$?"
	fi

	for extra_tag in ${extra_tags[@]}; do
		docker tag "${tag}" "${extra_tag}" || return 1
	done
}
readonly -f os::build::image::internal::docker
