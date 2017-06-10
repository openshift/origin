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

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --service-catalog-config) SC_KUBECONFIG="${2:-}"; shift ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

API_SERVER_HOST="$(kubectl get services -n catalog | grep 'catalog-catalog-apiserver' | awk '{print $3}')"

[[ "${API_SERVER_HOST}" =~ ^[0-9.]+$ ]] \
  || error_exit 'Error when fetching service catalog API Server IP address.'

echo "Using API Server IP: ${API_SERVER_HOST}"

export KUBECONFIG="${SC_KUBECONFIG:-"${KUBECONFIG}"}"
echo "Using config file at: ${SC_KUBECONFIG}"
kubectl config set-credentials service-catalog-creds --username=admin --password=admin
kubectl config set-cluster service-catalog-cluster --server="http://${API_SERVER_HOST}:80"
kubectl config set-context service-catalog-ctx --cluster=service-catalog-cluster --user=service-catalog-creds
kubectl config use-context service-catalog-ctx

retry -n 10 \
  kubectl get brokers,serviceclasses,instances,bindings \
  || error_exit 'Issue listing resources from service catalog API server.'

echo 'Set up service catalog kubeconfig.'
