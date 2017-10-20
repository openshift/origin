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
    --fix-auth) FIX_CONFIGMAP=true ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

REGISTRY="${REGISTRY:-}"
VERSION="${VERSION:-"canary"}"
CERT_FOLDER="${CERT_FOLDER:-"/tmp/sc-certs/"}"
FIX_CONFIGMAP="${FIX_CONFIGMAP:-false}"

CONTROLLER_MANAGER_IMAGE="${REGISTRY}controller-manager:${VERSION}"
APISERVER_IMAGE="${REGISTRY}apiserver:${VERSION}"

echo 'INSTALLING SERVICE CATALOG'
echo '-------------------'
echo "Using controller-manager image: ${CONTROLLER_MANAGER_IMAGE}"
echo "Using apiserver image: ${APISERVER_IMAGE}"
echo '-------------------'

# Create certificates for API server
echo 'Creating API server CA and certificate...'

# The SC_SERVING_CA, SC_SERVING_CERT, and SC_SERVING_KEY environment variables
# are sourced from this script.
CERT_FOLDER="${CERT_FOLDER}" source ${ROOT}/contrib/svc-cat-apiserver-aggregation-tls-setup.sh \
  || error_exit 'Error creating certificates for API server.'


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

PARAMETERS="$(cat <<-EOF
  --set apiserver.auth.enabled=true \
  --set useAggregator=true \
  --set apiserver.tls.ca=$(base64 --wrap 0 ${SC_SERVING_CA}) \
  --set apiserver.tls.cert=$(base64 --wrap 0 ${SC_SERVING_CERT}) \
  --set apiserver.tls.key=$(base64 --wrap 0 ${SC_SERVING_KEY}) \
  --set controllerManager.image=${CONTROLLER_MANAGER_IMAGE} \
  --set apiserver.image=${APISERVER_IMAGE}
EOF
)"

retry \
    helm install "${ROOT}/charts/catalog" \
    --name "catalog" \
    --namespace "catalog" \
    ${PARAMETERS} \
  || error_exit 'Error deploying service catalog to cluster.'

# Waiting for everything to come up

echo 'Waiting for Service Catalog API Server to be up...'

retry &> /dev/null \
  kubectl get clusterservicebrokers,clusterserviceclasses,serviceinstances,servicebindings \
  || {
    API_SERVER_POD_NAME="$(kubectl get pods --namespace catalog | grep catalog-catalog-apiserver | awk '{print $1}')"
    kubectl describe pod "${API_SERVER_POD_NAME}" --namespace catalog
    error_exit 'Timed out waiting for expected response from service catalog API server.'
  }

echo 'Waiting for Service Catalog Controller to be up...'

CONTROLLER_POD_NAME="$(kubectl get pods --namespace catalog | grep catalog-catalog-controller | awk '{print $1}')"
wait_for_expected_output -e 'Running' \
  kubectl get pods "${CONTROLLER_POD_NAME}" --namespace catalog \
  || {
    kubectl describe pod "${CONTROLLER_POD_NAME}" --namespace catalog
    error_exit 'Timed out waiting for service catalog controller-manager pod to come up.'
  }

echo 'Service Catalog installed successfully.'
