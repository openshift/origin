#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

pushd ~
  if [ ! -d "sti-basicauthurl" ]; then
    git clone https://github.com/openshift/sti-basicauthurl
  fi

  cd sti-basicauthurl
  git pull

  docker build --no-cache=true -t openshift3_beta/sti-basicauthurl .
  docker tag -f openshift3_beta/sti-basicauthurl localhost:5000/openshift3_beta/sti-basicauthurl
  docker push localhost:5000/openshift3_beta/sti-basicauthurl:latest
popd

docker rmi $(docker images -q --filter "dangling=true")
