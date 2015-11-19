#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

KUBE_ROOT=${1:-""}
KUBE_GODEP_ROOT="${OS_ROOT}/Godeps/_workspace/src/k8s.io/kubernetes"

if [ -z "$KUBE_ROOT" ]; then
  echo "usage: copy-kube-artifacts.sh <kubernetes root dir>"
  exit 255
fi

special_files="README.md
api/swagger-spec/v1.json
docs/user-guide/multi-pod.yaml
examples/examples_test.go
examples/pod
examples/iscsi/README.md
docs/user-guide/walkthrough/README.md
docs/user-guide/simple-yaml.md
pkg/client/testdata/myCA.cer
pkg/client/testdata/myCA.key
pkg/client/testdata/mycertvalid.cer
pkg/client/testdata/mycertvalid.key
pkg/client/testdata/mycertvalid.req
"

descriptor_dirs="cmd/integration
docs/admin/
docs/admin/limitrange/
docs/admin/namespaces/
docs/admin/resourcequota/
docs/user-guide/
docs/user-guide/downward-api/
docs/user-guide/downward-api/volume/
docs/user-guide/liveness/
docs/user-guide/logging-demo/
docs/user-guide/node-selection/
docs/user-guide/persistent-volumes/claims/
docs/user-guide/persistent-volumes/simpletest/
docs/user-guide/persistent-volumes/volumes/
docs/user-guide/secrets/
docs/user-guide/update-demo/
docs/user-guide/walkthrough/
examples/
examples/cephfs/
examples/elasticsearch/
examples/experimental/
examples/fibre_channel/
examples/guestbook
examples/guestbook-go
examples/iscsi
examples/glusterfs
examples/rbd/secret
examples/rbd
examples/cassandra
examples/celery-rabbitmq
examples/cluster-dns
examples/elasticsearch
examples/explorer
examples/hazelcast
examples/javaweb-tomcat-sidecar/
examples/meteor
examples/mysql-wordpress-pd
examples/nfs
examples/openshift-origin
examples/phabricator
examples/redis
examples/rethinkdb
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