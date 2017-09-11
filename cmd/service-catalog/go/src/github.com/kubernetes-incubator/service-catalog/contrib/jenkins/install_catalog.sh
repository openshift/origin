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
    --registry) REGISTRY="${2:-}"; shift ;;
    --version)  VERSION="${2:-}"; shift ;;
    --with-tpr) WITH_TPR=true ;;
    --fix-auth) FIX_CONFIGMAP=true ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

REGISTRY="${REGISTRY:-}"
VERSION="${VERSION:-"canary"}"
WITH_TPR="${WITH_TPR:-false}"
FIX_CONFIGMAP="${FIX_CONFIGMAP:-false}"

CONTROLLER_MANAGER_IMAGE="${REGISTRY}controller-manager:${VERSION}"
APISERVER_IMAGE="${REGISTRY}apiserver:${VERSION}"

echo 'INSTALLING SERVICE CATALOG'
echo '-------------------'
echo "Using controller-manager image: ${CONTROLLER_MANAGER_IMAGE}"
echo "Using apiserver image: ${APISERVER_IMAGE}"
echo '-------------------'

# Deploying to cluster

echo 'Deploying service catalog...'

retry \
    kubectl --namespace kube-system get configmap extension-apiserver-authentication \
  || error_exit 'Timed out waiting for extension-apiserver-authentication configmap to come up.'

# The API server automatically provisions the configmap, but we need it to contain the requestheader CA as well,
# which is only provisioned if you pass the appropriate flags to master.  GKE doesn't do this, so we work around it.
if [[ "${FIX_CONFIGMAP}" == true ]] && [[ -z "$(kubectl --namespace kube-system get configmap extension-apiserver-authentication -o jsonpath="{ $.data['requestheader-client-ca-file'] }")" ]]; then
    full_configmap=$(kubectl --namespace kube-system get configmap extension-apiserver-authentication -o json)
    echo "$full_configmap" | jq '.data["requestheader-client-ca-file"] = .data["client-ca-file"]' | kubectl --namespace kube-system update configmap extension-apiserver-authentication -f -
    [[ -n "$(kubectl --namespace kube-system get configmap extension-apiserver-authentication -o jsonpath="{ $.data['requestheader-client-ca-file'] }")" ]] || { echo "Could not add requestheader auth CA to extension-apiserver-authentication configmap."; exit 1; }
fi

VALUES=()
VALUES+="controllerManager.image=${CONTROLLER_MANAGER_IMAGE}"
VALUES+=",apiserver.image=${APISERVER_IMAGE}"
VALUES+=",apiserver.service.type=NodePort"
VALUES+=",apiserver.service.nodePort.securePort=30443"
if [[ "${WITH_TPR}" == true ]]; then
  VALUES+=',apiserver.storage.type=tpr'
  VALUES+=',apiserver.storage.tpr.globalNamespace=test-ns'
fi

retry \
    helm install "${ROOT}/charts/catalog" \
    --name "catalog" \
    --namespace "catalog" \
    --set "${VALUES}" \
  || error_exit 'Error deploying service catalog to cluster.'

# Waiting for everything to come up

echo 'Waiting on pods to come up...'

wait_for_expected_output -e 'catalog-catalog-controller-manager' \
    kubectl get pods --namespace catalog \
  && wait_for_expected_output -e 'catalog-catalog-apiserver' \
    kubectl get pods --namespace catalog \
  && wait_for_expected_output -x -e 'Pending' \
    kubectl get pods --namespace catalog \
  && wait_for_expected_output -x -e 'ContainerCreating' \
    kubectl get pods --namespace catalog \
  || error_exit 'Timed out waiting for service catalog pods to come up.'

[[ "$(kubectl get pods --namespace catalog | grep catalog-catalog-apiserver | awk '{print $3}')" == 'Running' ]] \
  || {
    POD_NAME="$(kubectl get pods --namespace catalog | grep catalog-catalog-apiserver | awk '{print $1}')"
    kubectl get pod "${POD_NAME}" --namespace catalog
    kubectl describe pod "${POD_NAME}" --namespace catalog
    error_exit 'API server pod did not come up successfully.'
  }

[[ "$(kubectl get pods --namespace catalog | grep catalog-catalog-controller | awk '{print $3}')" == 'Running' ]] \
  || {
    POD_NAME="$(kubectl get pods --namespace catalog | grep catalog-catalog-controller | awk '{print $1}')"
    kubectl get pod "${POD_NAME}" --namespace catalog
    kubectl describe pod "${POD_NAME}" --namespace catalog
    error_exit 'Controller manager pod did not come up successfully.'
  }

echo 'Waiting for Service Catalog API Server to be up...'

${ROOT}/contrib/jenkins/setup-sc-context.sh \
  || error_exit 'Error when setting up context for service catalog.'

retry &> /dev/null \
  kubectl --context=service-catalog get servicebrokers,serviceclasses,serviceinstances,serviceinstancecredentials \
  || error_exit 'Timed out waiting for expected response from service catalog API server.'

echo 'Service Catalog installed successfully.'
