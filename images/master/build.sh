#!/bin/sh

export DOCKER_IMAGE_NAME=aos-master
export DOCKER_IMAGE_VERSION=3.0.1.0-0
export DOCKER_REGISTRY=localhost:5000

#../build-common.sh

# From build-common in original oseonatomic repo:
#sudo docker build --no-cache=true --force-rm --tag $DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION .
sudo docker build -f Dockerfile.product --tag $DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION .
#docker tag --force $DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION $DOCKER_IMAGE_NAME:latest
#docker tag --force $DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION $DOCKER_REGISTRY/$DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION
#docker tag --force $DOCKER_IMAGE_NAME:$DOCKER_IMAGE_VERSION $DOCKER_REGISTRY/$DOCKER_IMAGE_NAME:latest

#docker push $DOCKER_REGISTRY/$DOCKER_IMAGE_NAME:latest


