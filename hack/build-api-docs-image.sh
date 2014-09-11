#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

cd $hackdir/../api && docker build -t kubernetes/raml2html .
docker rm oov3docgen &>/dev/null || :
docker run --name=oov3docgen kubernetes/raml2html
docker cp oov3docgen:/data/oov3.html $hackdir/../api/
docker rm oov3docgen &>/dev/null || :
