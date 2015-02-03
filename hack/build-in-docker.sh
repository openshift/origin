#!/bin/bash

# This script build the sources in openshift/origin-release image using
# the Fedora environment and Go compiler.

set -o errexit
set -o nounset
set -o pipefail

origin_path="src/github.com/openshift/origin"
docker run -v ${GOPATH}/${origin_path}:/go/${origin_path} openshift/origin-release /opt/bin/build.sh
