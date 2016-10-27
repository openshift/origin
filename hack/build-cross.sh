#!/bin/bash

# Build all cross compile targets and the base binaries
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

platforms=( "${OS_CROSS_COMPILE_PLATFORMS[@]}" )
if [[ -n "${OS_ONLY_BUILD_PLATFORMS-}" ]]; then
  filtered=()
  for platform in ${platforms[@]}; do
    if [[ "${platform}" =~ "${OS_ONLY_BUILD_PLATFORMS}" ]]; then
      filtered+=("${platform}")
    fi
  done
  platforms=("${filtered[@]}")
fi

# Build the primary client/server for all platforms
OS_BUILD_PLATFORMS=("${platforms[@]}")
host_platform=$(os::build::host_platform)
if [[ $host_platform == "linux/ppc64le" ]]; then
  OS_GOFLAGS_LINUX_PPC64LE="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
else
  OS_GOFLAGS_LINUX_AMD64="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
fi

# Build image binaries for a subset of platforms. Image binaries are currently
# linux-only, and are compiled with flags to make them static for use in Docker
# images "FROM scratch".
OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]-}")
# Pass the necessary tags
OS_GOFLAGS="${OS_GOFLAGS:-} ${OS_IMAGE_COMPILE_GOFLAGS}" os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS[@]-}" "${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]-}"

# Make the primary client/server release.
OS_RELEASE_ARCHIVE="openshift-origin"
OS_BUILD_PLATFORMS=("${platforms[@]}")
os::build::place_bins "${OS_CROSS_COMPILE_BINARIES[@]}"
if [[ "${OS_GIT_TREE_STATE:-dirty}" == "clean"  ]]; then
	# only when we are building from a clean state can we claim to
	# have created a valid set of binaries that can resemble a release
	echo "${OS_GIT_COMMIT}" > "${OS_LOCAL_RELEASEPATH}/.commit"
fi

# Make the image binaries release.
OS_RELEASE_ARCHIVE="openshift-origin-image"
OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]-}")
os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"

os::build::release_sha

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
