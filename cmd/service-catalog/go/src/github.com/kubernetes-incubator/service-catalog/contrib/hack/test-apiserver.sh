#!/bin/bash
# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export PATH=${ROOT}/contrib/hack:${PATH}

trap cleanup EXIT

function cleanup {
    rc=$?
	echo Cleaning up
	stop-server.sh
	exit $rc
}

start-server.sh

PORT=$(docker port etcd-svc-cat 8081 | sed "s/.*://")
D_HOST=${DOCKER_HOST:-localhost}
D_HOST=${D_HOST#*//}   # remove leading proto://
D_HOST=${D_HOST%:*}    # remove trailing port #
NO_TTY=1 kubectl config set-cluster service-catalog-cluster --server=https://${D_HOST}:${PORT} --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt

# create a few resources
set -x
NO_TTY=1 kubectl create -f contrib/examples/apiserver/broker.yaml
NO_TTY=1 kubectl create -f contrib/examples/apiserver/serviceclass.yaml
NO_TTY=1 kubectl create -f contrib/examples/apiserver/instance.yaml
NO_TTY=1 kubectl create -f contrib/examples/apiserver/binding.yaml

NO_TTY=1 kubectl get broker test-broker -o yaml
NO_TTY=1 kubectl get serviceclass test-serviceclass -o yaml
NO_TTY=1 kubectl get instance test-instance --namespace test-ns -o yaml
NO_TTY=1 kubectl get binding test-binding --namespace test-ns -o yaml

NO_TTY=1 kubectl delete -f contrib/examples/apiserver/broker.yaml
NO_TTY=1 kubectl delete -f contrib/examples/apiserver/serviceclass.yaml
NO_TTY=1 kubectl delete -f contrib/examples/apiserver/instance.yaml
NO_TTY=1 kubectl delete -f contrib/examples/apiserver/binding.yaml
set +x
