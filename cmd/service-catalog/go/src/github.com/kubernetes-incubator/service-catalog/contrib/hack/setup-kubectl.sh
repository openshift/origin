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

trap cleanup EXIT

function cleanup {
	rc=$?
	echo Cleaning up
	docker rm -f etcd-svc-cat apiserver > /dev/null 2>&1 || true
	exit $rc
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export PATH=${ROOT}/contrib/hack:${PATH}

# Make our kubectl image, if not already there
make-kubectl.sh

# Stop any existing sever
stop-server.sh > /dev/null 2>&1 || true

# Start the API Server so that we can setup our kubectl env
start-server.sh

# Find the port # that Docker assigned to the server
PORT=$(docker port etcd-svc-cat 443 | sed "s/.*://")

D_HOST=${DOCKER_HOST:-localhost}
D_HOST=${D_HOST#*//}   # remove leading proto://
D_HOST=${D_HOST%:*}    # remove trailing port #

# Setup our credentials
NO_TTY=1 kubectl config set-credentials service-catalog-creds --username=admin --password=admin
#NO_TTY=1 kubectl config set-cluster service-catalog-cluster --server=https://${D_HOST}:${PORT} --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt
NO_TTY=1 kubectl config set-cluster service-catalog-cluster --server=https://${D_HOST}:${PORT}
NO_TTY=1 kubectl config set-context service-catalog-ctx --cluster=service-catalog-cluster --user=service-catalog-creds
NO_TTY=1 kubectl config use-context service-catalog-ctx
