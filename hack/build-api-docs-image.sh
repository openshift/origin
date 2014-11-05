#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

cd "${OS_ROOT}/api"
docker build -t kubernetes/raml2html .
docker rm openshift3docgen &>/dev/null || :
docker run --name=openshift3docgen kubernetes/raml2html
docker cp openshift3docgen:/data/openshift3.html ${OS_ROOT}/api/
docker rm openshift3docgen &>/dev/null || :
