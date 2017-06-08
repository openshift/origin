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

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --prefix)   PREFIX="${2:-}"; shift ;;
    --location) LOCATION="${2:-}"; shift ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

PREFIX="${PREFIX:-}"
LOCATION="${LOCATION:-"${ROOT}"}"

# Deployments
kubectl describe deployments -n catalog catalog-catalog-apiserver > "${LOCATION}/${PREFIX}_apiserver_deployment.txt"
kubectl describe deployments -n catalog catalog-catalog-controller-manager > "${LOCATION}/${PREFIX}_controller-manager_deployment.txt"

# Pods
API_SERVER_POD="$(kubectl get pods -n catalog | grep 'catalog-catalog-apiserver' | awk '{print $1}')"
CONTROLLER_MANAGER_POD="$(kubectl get pods -n catalog | grep 'catalog-catalog-controller-manager' | awk '{print $1}')"
kubectl describe pods -n catalog "${API_SERVER_POD}" > "${LOCATION}/${PREFIX}_apiserver_pod.txt"
kubectl describe pods -n catalog "${CONTROLLER_MANAGER_POD}" > "${LOCATION}/${PREFIX}_controller-manager_pod.txt"

# Containers
kubectl logs -n catalog "${API_SERVER_POD}" etcd > "${LOCATION}/${PREFIX}_apiserver_etcd_container.txt"
kubectl logs -n catalog "${API_SERVER_POD}" apiserver > "${LOCATION}/${PREFIX}_apiserver_apiserver_container.txt"
kubectl logs -n catalog "${CONTROLLER_MANAGER_POD}" > "${LOCATION}/${PREFIX}_controller-manager_container.txt"
