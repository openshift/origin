#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

cd $hackdir/../api && docker build -t kubernetes/raml2html .
docker rm openshift3docgen &>/dev/null || :
docker run --name=openshift3docgen kubernetes/raml2html
docker cp openshift3docgen:/data/openshift3.html $hackdir/../api/
docker rm openshift3docgen &>/dev/null || :
