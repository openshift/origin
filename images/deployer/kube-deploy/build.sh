#!/bin/sh
KUBE_DEPLOY_DIR=$(dirname $(readlink -f 0))
cd $KUBE_DEPLOY_DIR
source ../../../hack/config-go.sh
CGO_ENABLED=0 go build -a -ldflags '-s' cmd/kube-deploy.go
