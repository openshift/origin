#!/bin/bash

# This script builds all images of components to be integrated into OpenShift as
# core services (e.g. logging, metric)

set -o errexit
set -o nounset
set -o pipefail

tag=${OS_TAG:-""}

if [[ -z "$tag" ]]; then
  echo "You must specify the OS_TAG variable to checkout the correct release commit for the integration services images, e.g. 'v1.0.1'."
  exit 1
fi

STARTTIME=$(date +%s)

function build_n_tag_images {
  repo=https://github.com/openshift/${1}.git
  echo "Building integration image from source $repo ..."
  clonedir="$(mktemp -d 2>/dev/null || mktemp -d -t clonedir.XXXXXX)"
  git clone -q -b $tag --depth 1 --single-branch $repo $clonedir
  pushd $clonedir/hack
  ./build-images.sh --version=$tag
  popd
}

build_n_tag_images origin-aggregated-logging
build_n_tag_images origin-metrics

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
