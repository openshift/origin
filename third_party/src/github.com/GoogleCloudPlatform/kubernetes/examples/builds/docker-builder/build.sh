#!/bin/bash
set -uo pipefail
IFS=$'\n\t'

NEED_DIND=false
if [ ! -e /var/run/docker.sock ]; then
  NEED_DIND=true
fi

if $NEED_DIND; then
  DOCKER_READY=false
  dind &

  # wait for docker to be available
  ATTEMPTS=0
  while [ $ATTEMPTS -lt 10 ]; do
    docker version &> /dev/null
    if [ $? -eq 0 ]; then
      DOCKER_READY=true
      break
    fi

    let ATTEMPTS=ATTEMPTS+1
    sleep 1
  done

  if ! $DOCKER_READY; then
    echo 'Docker-in-Docker daemon not accessible'
    exit 1
  fi
fi

TAG=$BUILD_TAG
if [ -n "$DOCKER_REGISTRY" ]; then
  TAG=$DOCKER_REGISTRY/$BUILD_TAG
fi

docker build --rm -t $TAG $DOCKER_CONTEXT_URL

if [ -n "$DOCKER_REGISTRY" ]; then
  docker push $TAG
fi

if $NEED_DIND; then
  kill -15 $(cat /var/run/docker.pid)
fi
