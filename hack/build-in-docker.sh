#!/bin/bash

# This script build the sources in openshift/origin-release image using
# the Fedora environment and Go compiler.
function absolute_path() {
  pushd . > /dev/null
  [ -d "$1" ] && cd "$1" && dirs -l +0
  popd > /dev/null
}

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
	return_code=$?
	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

origin_path="src/github.com/openshift/origin"

docker run -e "OWNER_GROUP=$UID:$GROUPS" --rm -v "$(absolute_path $OS_ROOT):/go/${origin_path}:z" openshift/origin-release /usr/bin/openshift-origin-build.sh $@
