#!/bin/bash

# Build all compile targets and the base binaries for only the platform of the
# host it's building on. (i.e. - no cross compile)
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

source <(go env)
platforms=( "${GOHOSTOS}/${GOARCH}" )
if [[ -n "${OS_ONLY_BUILD_PLATFORMS:-}" ]]; then
  filtered=()
  for platform in "${platforms[@]}"; do
    if [[ "${platform}" =~ "${OS_ONLY_BUILD_PLATFORMS}" ]]; then
      filtered+=("${platform}")
    fi
  done
  platforms=("${filtered[@]}")
fi

# Build the primary client/server for the host platform
OS_BUILD_PLATFORMS=("${platforms[@]}")
case "${GOHOSTOS}/${GOARCH}" in
  "linux/amd64")
    OS_GOFLAGS_LINUX_AMD64="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
    ;;
  "linux/arm64")
    OS_GOFLAGS_LINUX_ARM64="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
    ;;
  "linux/386")
    OS_GOFLAGS_LINUX_386="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
    ;;
  "linux/arm")
    OS_GOFLAGS_LINUX_ARM="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
    ;;
  "linux/ppc64")
    OS_GOFLAGS_LINUX_PPC64="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
    ;;
  "linux/ppc64le")
    OS_GOFLAGS_LINUX_PPC64LE="-tags=gssapi" os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"
    ;;
esac

# Pass the necessary tags
OS_GOFLAGS="${OS_GOFLAGS:-} ${OS_IMAGE_COMPILE_GOFLAGS}" os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS[@]:-}" "${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]:-}"

# Make the primary client/server release.
OS_RELEASE_ARCHIVE="openshift-origin"
os::build::place_bins "${OS_CROSS_COMPILE_BINARIES[@]}"

# Make the image binaries release.
OS_RELEASE_ARCHIVE="openshift-origin-image"
os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
