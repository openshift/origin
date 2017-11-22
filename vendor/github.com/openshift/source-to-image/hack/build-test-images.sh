#!/bin/bash

# This script builds all images locally (requires Docker)

set -o errexit
set -o nounset
set -o pipefail

readonly S2I_ROOT=$(dirname "${BASH_SOURCE}")/..

s2i::build_test_image() {
  local image_name="$1"
  local tag="sti_test/${image_name}"
  local src="test/integration/images/${image_name}"
  cp -R test/integration/scripts "${src}"
  docker build -t "${tag}" "${src}"
  rm -rf "${src}/scripts"
}

(
  # Go to the top of the tree.
  cd "${S2I_ROOT}"

  s2i::build_test_image sti-fake
  s2i::build_test_image sti-fake-env
  s2i::build_test_image sti-fake-user
  s2i::build_test_image sti-fake-scripts
  s2i::build_test_image sti-fake-scripts-no-save-artifacts
  s2i::build_test_image sti-fake-no-tar
  s2i::build_test_image sti-fake-onbuild
  s2i::build_test_image sti-fake-numericuser
  s2i::build_test_image sti-fake-onbuild-rootuser
  s2i::build_test_image sti-fake-onbuild-numericuser
)
