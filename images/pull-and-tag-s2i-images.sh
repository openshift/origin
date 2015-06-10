#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

os::build::get_version_vars
# OS_GIT_VERSION is like 'v0.3.1-4-g2c853ed-dirty'
IMAGE_VERSION=`echo ${OS_GIT_VERSION} | cut -d '-' -f 1`

s2i_images="mysql-55-rhel7 ruby-20-rhel7 python-33-rhel7 php-55-rhel7 perl-516-rhel7 nodejs-010-rhel7 mongodb-24-rhel7 postgresql-92-rhel7"
s2i_src_repo="ci.dev.openshift.redhat.com:5000/openshift/"
s2i_dst_repo="ci.dev.openshift.redhat.com:5000/openshift3/"

for img in $s2i_images;
do
   docker pull ${s2i_src_repo}${img}
   docker tag -f ${s2i_src_repo}${img}:latest ${s2i_dst_repo}${img}:${IMAGE_VERSION}
done
