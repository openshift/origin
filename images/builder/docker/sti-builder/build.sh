#!/bin/bash -ex

DOCKER_SOCKET=/var/run/docker.sock

if [ ! -e $DOCKER_SOCKET ]; then
  echo "Docker socket missing at $DOCKER_SOCKET"
  exit 1
fi

TAG=$BUILD_TAG
if [ -n "$DOCKER_REGISTRY" ]; then
  TAG=$DOCKER_REGISTRY/$BUILD_TAG
fi

REF_OPTION=""
if [ -n "$SOURCE_REF" ]; then
  REF_OPTION="--ref $SOURCE_REF"
fi

BUILD_TEMP_DIR=${TEMP_DIR-$TMPDIR}
TMPDIR=$BUILD_TEMP_DIR sti build $SOURCE_URI $BUILDER_IMAGE $TAG $REF_OPTION

if [ -n "$DOCKER_REGISTRY" ] || [ -s "/root/.dockercfg" ]; then
  docker push $TAG
fi
