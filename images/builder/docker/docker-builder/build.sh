#!/bin/bash
set -uo pipefail
IFS=$'\n\t'

DOCKER_SOCKET=/var/run/docker.sock

if [ ! -e $DOCKER_SOCKET ]; then
  echo "Docker socket missing at $DOCKER_SOCKET"
  exit 1
fi

TAG=$BUILD_TAG
if [ -n "$DOCKER_REGISTRY" ]; then
  TAG=$DOCKER_REGISTRY/$BUILD_TAG
fi

docker build --rm -t $TAG $DOCKER_CONTEXT_URL

if [ -n "$DOCKER_REGISTRY" ]; then
  docker push $TAG
fi
