#!/bin/bash

set -x

TEST_IMAGE=atomic-registry-install
HOST=${1:-"localhost"}

docker build -t ${TEST_IMAGE} .

IMAGES=(openshift/origin openshift/origin-docker-registry cockpit/kubernetes)
for IMAGE in "${IMAGES[@]}"
do
  docker pull $IMAGE
done

atomic install ${TEST_IMAGE} ${HOST}
systemctl enable --now atomic-registry-master.service
sleep 10
sudo /var/run/setup-atomic-registry.sh ${HOST}
