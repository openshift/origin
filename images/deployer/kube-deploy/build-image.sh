#!/bin/sh
KUBE_DEPLOY_DIR=$(dirname $(readlink -f 0))
cd $KUBE_DEPLOY_DIR
DOCKERUSER=${DOCKERUSER:-openshift}
sudo docker build -t $DOCKERUSER/kube-deploy .
rm kube-deploy
