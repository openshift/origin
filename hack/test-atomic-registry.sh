#!/bin/bash

set -o errexit
set -o pipefail

IMAGE=atomic-registry-quickstart
docker build -t $IMAGE ../images/atomic-registry-quickstart/.

set -x

function install() {
  INSTALL=$(docker inspect -f '{{ .ContainerConfig.Labels.INSTALL }}' $IMAGE)
  # We need $IMAGE string replaced with the image we built here
  ${INSTALL//\$IMAGE/$IMAGE}
}

function run() {
  RUN=$(docker inspect -f '{{ .ContainerConfig.Labels.RUN }}' $IMAGE)
  ${RUN//\$IMAGE/$IMAGE}
}

function stop() {
  STOP=$(docker inspect -f '{{ .ContainerConfig.Labels.STOP }}' $IMAGE)
  ${STOP//\$IMAGE/$IMAGE}
}

function uninstall() {
  UNINSTALL=$(docker inspect -f '{{ .ContainerConfig.Labels.UNINSTALL }}' $IMAGE)
  ${UNINSTALL//\$IMAGE/$IMAGE}
}

if [ ! -z $1 ] ; then
  $1
  exit
fi

install
run
stop
uninstall

