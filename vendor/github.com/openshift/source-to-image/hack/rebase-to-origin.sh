#!/bin/bash
#
# This script will create a rebase commit in the OpenShift Origin Git repository
# based on the current HEAD.
#
# NOTE: Make sure all your changes are committed and there are no junk files
#       present in the pkg/ folder.
#

set -o errexit
set -o nounset
set -o pipefail

readonly S2I_ROOT=$(
  root=$(dirname "${BASH_SOURCE}")/..
  unset CDPATH
  cd "${root}"
  pwd
)
#readonly OS_ROOT="${S2I_ROOT/%\/source-to-image/\/ose}"
readonly OS_ROOT="${S2I_ROOT/%\/source-to-image/\/origin}"

source "${S2I_ROOT}/hack/util.sh"

# Exclude these packages from source-to-image explicitly
readonly exclude_pkgs=(
  pkg/cmd
  pkg/config
  pkg/create
  pkg/docker/test
  pkg/run
  pkg/version
  pkg/test
)

readonly origin_s2i_vendor_dir="${OS_ROOT}/vendor/github.com/openshift/source-to-image"
readonly s2i_ref="$(git -C ${S2I_ROOT} rev-parse --verify HEAD)"
readonly s2i_short_ref="$(git -C ${S2I_ROOT} rev-parse --short HEAD)"
readonly s2i_describe_tags="$(git -C ${S2I_ROOT} describe --tags HEAD)"
readonly s2i_godeps_ref="$(grep -m1 -A2 'openshift/source-to-image' ${OS_ROOT}/Godeps/Godeps.json |
  grep Rev | cut -d ':' -f2 | sed -e 's/"//g' -e 's/^[[:space:]]*//')"
readonly s2i_godeps_comment="$(grep -m1 -A2 'openshift/source-to-image' ${OS_ROOT}/Godeps/Godeps.json |
  grep Comment | cut -d ':' -f2 | sed -e 's/[",]//g' -e 's/^[[:space:]]*//')"

pushd "${OS_ROOT}" >/dev/null
  git checkout -B "s2i-${s2i_short_ref}-bump" master
  rm -rf "${origin_s2i_vendor_dir}"/*
  cp -R "${S2I_ROOT}/pkg" "${origin_s2i_vendor_dir}/."
  cp "${S2I_ROOT}/LICENSE" "${origin_s2i_vendor_dir}/."
  # remove test files from the vendor folder.
  find ${origin_s2i_vendor_dir}/pkg -name "*_test.go" -delete
  # Remove all explicitly excluded packages
  for pkg in "${exclude_pkgs[@]}"; do
    rm -rvf "${origin_s2i_vendor_dir}/${pkg}"
  done

  # Bump the origin Godeps.json file
  s2i::util::sed "s/${s2i_godeps_ref}/${s2i_ref}/g" "${OS_ROOT}/Godeps/Godeps.json"
  s2i::util::sed "s/${s2i_godeps_comment}/${s2i_describe_tags}/g" "${OS_ROOT}/Godeps/Godeps.json"

  # Make a commit with proper message
  git add Godeps vendor && git commit -m "bump(github.com/openshift/source-to-image): ${s2i_ref}"
popd
