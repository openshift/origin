#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

KUBE_ROOT=${1:-""}
KUBE_GODEP_ROOT="${OS_ROOT}/Godeps/_workspace/src/github.com/GoogleCloudPlatform/kubernetes"

if [ -z "$KUBE_ROOT" ]; then
  echo "usage: copy-kube-artifacts.sh <kubernetes root dir>"
  exit 255
fi

special_files="README.md
api/swagger-spec/v1.json
examples/examples_test.go
examples/walkthrough/README.md
examples/iscsi/README.md
examples/simple-yaml.md
"

descriptor_dirs="cmd/integration
examples/
examples/guestbook
examples/guestbook-go
examples/walkthrough
examples/update-demo
examples/persistent-volumes/volumes
examples/persistent-volumes/claims
examples/persistent-volumes/simpletest
examples/iscsi
examples/glusterfs
examples/liveness
examples/rbd/secret
examples/rbd
examples/cassandra
examples/celery-rabbitmq
examples/cluster-dns
examples/downward-api
examples/elasticsearch
examples/explorer
examples/hazelcast
examples/kubernetes-namespaces
examples/limitrange
examples/logging-demo
examples/meteor
examples/mysql-wordpress-pd
examples/nfs
examples/node-selection
examples/openshift-origin
examples/phabricator
examples/redis
examples/resourcequota
examples/rethinkdb
examples/secrets
examples/spark
examples/storm"

for file in $special_files
do
  dir=`dirname $file`
  mkdir -p $KUBE_GODEP_ROOT/$dir

  cp -v $KUBE_ROOT/$file $KUBE_GODEP_ROOT/$file
done

for dir in $descriptor_dirs
do
  mkdir -p $KUBE_GODEP_ROOT/$dir
  files_to_copy=`find $KUBE_ROOT/$dir -maxdepth 1 -name '*.json' -o -name '*.yaml'`

  for file in $files_to_copy
  do
    cp -vf $file $KUBE_GODEP_ROOT/$dir
  done
done