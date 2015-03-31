#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail


if ! [[ ${1} =~ v([0-9]+\.[0-9]+) ]];
then
  echo "Usage ${0} v0.1";
  exit 1
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
VERSION_LDFLAGS=$(os::build::ldflags)
os::build::os_version_vars
if [[ "${OS_GIT_VERSION}" =~ -dirty ]]; then
  echo "This command commits changes to tracked files and should not be run if your tree is dirty"
  exit 1
fi

RPM_VERSION=`echo ${1} | cut -b2-`
CHANGELOG_ENTRY="* `date +'%a %b %d %Y'` `git config user.name` `git config user.email` - ${RPM_VERSION}\n- Version ${RPM_VERSION}\n"
sed -i 's/^%global commit.*/%global commit '"${OS_GIT_COMMIT}"'/' openshift.spec
sed -i 's|^%global ldflags.*|%global ldflags '"${VERSION_LDFLAGS}"'|' openshift.spec
sed -i 's/^Version.*/Version:        '"${RPM_VERSION}"'/' openshift.spec
sed -i 's/^%changelog$/%changelog\n'"${CHANGELOG_ENTRY}"'/' openshift.spec
git commit -a -m "Version $1"
git tag $1 -a -m "$1" HEAD

