#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

hack_dir=$(CDPATH="" cd $(dirname $0); pwd)
api_dir=$hack_dir/../api

image_name=kubernetes/raml2html
image=$(docker images -q $image_name)

if [ -z "$image" ]; then
  echo "Building raml2html image"
  (cd $api_dir && docker build -t kubernetes/raml2html .)
fi

echo "Running raml2html"
docker run --rm -v $api_dir:/data --name=oov3docgen kubernetes/raml2html
