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

# Clean up old containers if still around
docker rm -f etcd-svc-cat apiserver > /dev/null 2>&1 || true

# Start etcd, our DB.
echo Starting etcd
# we map the port here (even though etcd doesn't use 443) because we
# can't map it later when we put the apiserver into the same network
# namespace as etcd
docker run --name etcd-svc-cat -p 443 -d quay.io/coreos/etcd > /dev/null
PORT=$(docker port etcd-svc-cat 443 | sed "s/.*://")

# And now our API Server
echo Starting the API Server
docker run -d --name apiserver \
	-v ${ROOT}:/go/src/github.com/kubernetes-incubator/service-catalog \
	-v ${ROOT}/.var/run/kubernetes-service-catalog:/var/run/kubernetes-service-catalog \
	-v ${ROOT}/.kube:/root/.kube \
	-e KUBERNETES_SERVICE_HOST=localhost \
	-e KUBERNETES_SERVICE_PORT=6443 \
	-e SERVICE_CATALOG_STANDALONE=true \
	--privileged \
	--net container:etcd-svc-cat \
	scbuildimage \
	bin/service-catalog apiserver -v 10 --etcd-servers http://localhost:2379 \
		--storage-type=etcd --disable-auth

# Wait for apiserver to be up and running
echo Waiting for API Server to be available...
count=0
D_HOST=${DOCKER_HOST:-localhost}
D_HOST=${D_HOST#*//}   # remove leading proto://
D_HOST=${D_HOST%:*}    # remove trailing port #
while ! wget --ca-certificate ${ROOT}/.var/run/kubernetes-service-catalog/apiserver.crt https://${D_HOST}:${PORT} > /dev/null 2>&1 ; do
	sleep 1
	(( count++ )) || true
	if [ "${count}" == "30" ]; then
		echo "Timed-out waiting for API Server"
		(set -x ; wget --ca-certificate ${ROOT}/.var/run/kubernetes-service-catalog/apiserver.crt https://${D_HOST}:${PORT})
		(set -x ; docker ps)
		(set -x ; docker logs apiserver)
		exit 1
	fi
done
echo API Server is ready
