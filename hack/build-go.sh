#!/bin/bash

# This script sets up a go workspace locally and builds all go components.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
	return_code=$?
	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

platform="$(os::build::host_platform)"

build_targets=("$@")
if [[ -z "$@" ]]; then
	build_targets=("${OS_CROSS_COMPILE_TARGETS[@]}")
  # Also build SDN components on Linux by default
  if [[ "${platform}" == linux/* ]]; then
    build_targets=("${build_targets[@]}" "${OS_SDN_COMPILE_TARGETS_LINUX[@]}")
  fi
fi

OS_BUILD_PLATFORMS=("${OS_BUILD_PLATFORMS[@]:-${platform}}")
os::build::build_binaries "${build_targets[@]}"
os::build::place_bins "${build_targets[@]}"
os::build::make_openshift_binary_symlinks
