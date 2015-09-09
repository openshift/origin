#!/bin/sh

export DOCKER_IMAGE_NAME=aos-node
export DOCKER_IMAGE_VERSION=3.0.1.0-0
export DOCKER_REGISTRY=localhost:5000

#../build-common.sh

sudo docker build -f Dockerfile.product --tag $DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION .

