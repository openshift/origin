#!/bin/bash -ex

NEED_DIND=false
if [ ! -e /var/run/docker.sock ]; then
  NEED_DIND=true
fi

if [ $NEED_DIND == "true" ]; then
  DOCKER_READY=false
  dind &

  # wait for docker to be available
  ATTEMPTS=0
  while [ $ATTEMPTS -lt 10 ]; do
    set +e
    docker version &> /dev/null
    if [ $? -eq 0 ]; then
      DOCKER_READY=true
      break
    fi
    set -e

    let ATTEMPTS=ATTEMPTS+1
    sleep 1
  done

  if [ $DOCKER_READY != "true" ]; then
    echo 'Docker-in-Docker daemon not accessible'
    exit 1
  fi
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

if [ -n "$DOCKER_REGISTRY" ]; then
  docker push $TAG
fi

if [ $NEED_DIND == "true" ]; then
  docker_pid=$(cat /var/run/docker.pid)
  kill -15 $docker_pid

  # wait up to 10 seconds for the Docker daemon to stop
  #
  # if it takes longer than that, something is probably wrong
  # and we may end up leaking loopback devices
  ATTEMPTS=0
  while [ $ATTEMPTS -lt 10 ]; do
    ps -p $docker_pid &> /dev/null
    if [ $? -eq 0 ]; then
      let ATTEMPTS=ATTEMPTS+1
      sleep 1
    else
      break
    fi
  done
fi
