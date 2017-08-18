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

set -o nounset
set -o errexit

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

. "${ROOT}/contrib/hack/utilities.sh" || { echo 'Cannot load bash utilities.'; exit 1; }

# If no IP provided, get the IP of one of the nodes of the cluster
if [[ -z "${IP:-}" ]]; then
  IP="$(kubectl get nodes -o json | jq '.items[0].status.addresses[] | if .type == "InternalIP" then .address else empty end' -r)"
  PORT=30443

  [[ "${IP}" =~ ^[0-9]+.[0-9]+.[0-9]+.[0-9]+$ ]] \
    || error_exit 'Error when fetching cluster node IP address.'
fi

echo "Using IP: ${IP}"

kubectl config set-credentials service-catalog-creds --username=admin --password=admin
kubectl config set-cluster service-catalog --server="https://${IP}:${PORT}" --insecure-skip-tls-verify=true
kubectl config set-context service-catalog --cluster=service-catalog --user=service-catalog-creds

echo 'Set up service catalog API server context.'
