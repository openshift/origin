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

if [[ $DOCKER_CONTEXT_URL != "git://"* ]] && [[ $DOCKER_CONTEXT_URL != "git@"* ]]; then
  URL=$DOCKER_CONTEXT_URL
  if [[ $URL != "http://"* ]] && [[ $URL != "https://"* ]]; then
    URL="https://"$URL
  fi
  curl --head --silent --fail --location --max-time 16 $URL > /dev/null
  if [ $? != 0 ]; then
    echo "Not found: "$DOCKER_CONTEXT_URL
    exit 1
  fi
fi

docker build --rm -t $TAG $DOCKER_CONTEXT_URL

if [ -n "$DOCKER_REGISTRY" ] || [ -s "/root/.dockercfg" ]; then
  docker push $TAG
fi
