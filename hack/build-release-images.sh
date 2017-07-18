#!/bin/bash

# This script builds the release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

ret=0

# IF OS_BUILD_ARCHES is not specified, default to the host architecture
build_arches="${OS_BUILD_ARCHES:-$(os::util::go_arch)}"

declare -A go_ver_arches
for ver in "${OS_BUILD_ENV_GOLANG_VERSIONS[@]}"; do
  declare -a arches
  for arch in ${build_arches}; do
    if [[ "${OS_BUILD_GOLANG_VERSION_ARCH_MAP["${ver}"]}" =~ "${arch}" ]]; then
      arches+=("${arch}")
    fi
  done
  if (( "${#arches[@]}" )); then
    go_ver_arches[$ver]="${arches[@]}"
  fi
  unset arches
done

for go_ver in ${!go_ver_arches[@]}; do
  image_arches="${go_ver_arches[${go_ver}]}"
  image_basename="${tag_prefix}-release"
  image_tag="golang-${go_ver}"
  image_dir="${OS_ROOT}/images/release/${image_tag}"

  os::build::cross_images ${image_basename} ${image_dir} ${image_arches} ${image_tag} || (ret=1 && break)
done

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
