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
    --registry)         REGISTRY="${2:-}"; shift ;;
    --version)          VERSION="${2:-}"; shift ;;
    --cleanup)          CLEANUP=true ;;
    --create-artifacts) CREATE_ARTIFACTS=true ;;
    --fix-auth)         FIX_CONFIGMAP=true ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

REGISTRY="${REGISTRY:-}"
VERSION="${VERSION:-"canary"}"
CLEANUP="${CLEANUP:-false}"
CREATE_ARTIFACTS="${CREATE_ARTIFACTS:-false}"
FIX_CONFIGMAP="${FIX_CONFIGMAP:-false}"

UPS_BROKER_IMAGE="${REGISTRY}user-broker:${VERSION}"

function cleanup() {
  if [[ "${CREATE_ARTIFACTS}" == true ]]; then
    echo 'Creating artifacts...'
    PREFIX='walkthrough'

    "${ROOT}/contrib/hack/create_artifacts.sh" \
        --prefix "${PREFIX}" --location "${ROOT}" \
        &> /dev/null \
      || true
  fi

  echo 'Cleaning up resources...'
  {
    helm delete --purge "ups-broker" || true
    helm delete --purge "catalog" || true
    kubectl delete secret -n test-ns my-secret || true
    kubectl delete namespace test-ns || true

    wait_for_expected_output -x -e 'test-ns' \
        kubectl get namespaces
  } &> /dev/null
}

if [[ "${CLEANUP}" == true ]]; then
  trap cleanup EXIT
fi

echo 'TESTING WALKTHROUGH'
echo '-------------------'
echo "Using ups-broker image: ${UPS_BROKER_IMAGE}"
echo '-------------------'

echo 'Creating "test-ns" namespace...'

kubectl create namespace test-ns \
  || error_exit 'Error creating "test-ns" namespace.'

# Deploy broker to cluster

echo 'Deploying user-provided-service broker...'

VALUES="image=${UPS_BROKER_IMAGE}"

retry \
    helm install "${ROOT}/charts/ups-broker" \
    --name "ups-broker" \
    --namespace "ups-broker" \
    --set "${VALUES}" \
  || error_exit 'Error deploying ups-broker to cluster.'

echo 'Waiting on pods to come up...'

wait_for_expected_output -e 'ups-broker-ups-broker' \
    kubectl get pods --namespace ups-broker \
  && wait_for_expected_output -x -e 'Pending' \
    kubectl get pods --namespace ups-broker \
  && wait_for_expected_output -x -e 'ContainerCreating' \
    kubectl get pods --namespace ups-broker \
  || error_exit 'Timed out waiting for user-provided-service broker pod to come up.'

[[ "$(kubectl get pods --namespace ups-broker | grep ups-broker-ups-broker | awk '{print $3}')" == 'Running' ]] \
  || {
    POD_NAME="$(kubectl get pods --namespace ups-broker | grep ups-broker-ups-broker | awk '{print $1}')"
    kubectl get pod "${POD_NAME}" --namespace ups-broker
    kubectl describe pod "${POD_NAME}" --namespace ups-broker
    error_exit 'User provided service broker pod did not come up successfully.'
  }

# Deploy service catalog

echo 'Deploying service catalog...'

FLAGS=()
[[ -n "${REGISTRY}" ]]           && FLAGS+="--registry ${REGISTRY} "
[[ -n "${VERSION}" ]]            && FLAGS+="--version ${VERSION} "
[[ "${FIX_CONFIGMAP}" == true ]] && FLAGS+="--fix-auth "

${ROOT}/contrib/jenkins/install_catalog.sh ${FLAGS} \
  || error_exit 'Could not install service catalog.'

# Create the broker

echo 'Creating broker...'

kubectl --context=service-catalog create -f "${ROOT}/contrib/examples/walkthrough/ups-broker.yaml" \
  || error_exit 'Error when creating ups-broker.'

wait_for_expected_output -e 'FetchedCatalog' \
    kubectl --context=service-catalog get servicebrokers ups-broker -o yaml \
  || {
    kubectl --context=service-catalog get servicebrokers ups-broker -o yaml
    error_exit 'Did not receive expected condition when creating ups-broker.'
  }

[[ "$(kubectl --context=service-catalog get servicebrokers ups-broker -o yaml)" == *"status: \"True\""* ]] \
  || {
    kubectl --context=service-catalog get servicebrokers ups-broker -o yaml
    error_exit 'Failure status reported when attempting to fetch catalog from ups-broker.'
  }

[[ "$(kubectl --context=service-catalog get serviceclasses)" == *user-provided-service* ]] \
  || error_exit 'user-provided-service not listed when fetching service classes.'

# Provision an instance

echo 'Provisioning instance...'

kubectl --context=service-catalog create -f "${ROOT}/contrib/examples/walkthrough/ups-instance.yaml" \
  || error_exit 'Error when creating ups-instance.'

wait_for_expected_output -e 'ProvisionedSuccessfully' \
  kubectl --context=service-catalog get serviceinstances -n test-ns ups-instance -o yaml \
  || {
    kubectl --context=service-catalog get serviceinstances -n test-ns ups-instance -o yaml
    error_exit 'Did not receive expected condition when provisioning ups-instance.'
  }

[[ "$(kubectl --context=service-catalog get serviceinstances -n test-ns ups-instance -o yaml)" == *"status: \"True\""* ]] \
  || {
    kubectl --context=service-catalog get serviceinstances -n test-ns ups-instance -o yaml
    error_exit 'Failure status reported when attempting to provision ups-instance.'
  }

# Bind to the instance

echo 'Binding to instance...'

kubectl --context=service-catalog create -f "${ROOT}/contrib/examples/walkthrough/ups-instance-credential.yaml" \
  || error_exit 'Error when creating ups-instance-credential.'

wait_for_expected_output -e 'InjectedBindResult' \
  kubectl --context=service-catalog get serviceinstancecredentials -n test-ns ups-instance-credential -o yaml \
  || {
    kubectl --context=service-catalog get serviceinstancecredentials -n test-ns ups-instance-credential -o yaml
    error_exit 'Did not receive expected condition when injecting ups-instance-credential.'
  }

[[ "$(kubectl --context=service-catalog get serviceinstancecredentials -n test-ns ups-instance-credential -o yaml)" == *"status: \"True\""* ]] \
  || {
    kubectl --context=service-catalog get serviceinstancecredentials -n test-ns ups-instance-credential -o yaml
    error_exit 'Failure status reported when attempting to inject ups-instance-credential.'
  }

[[ "$(kubectl get secrets -n test-ns)" == *ups-instance-credential* ]] \
  || error_exit '"ups-instance-credential" not present when listing secrets.'

#Unbind from the instance

echo 'Unbinding from instance...'

kubectl --context=service-catalog delete -n test-ns serviceinstancecredentials ups-instance-credential \
  || error_exit 'Error when deleting ups-instance-credential.'

wait_for_expected_output -x -e "ups-instance-credential" \
    kubectl get secrets -n test-ns \
  || error_exit '"ups-instance-credential" secret not removed upon deleting ups-instance-credential.'

# Deprovision the instance

echo 'Deprovisioning instance...'

kubectl --context=service-catalog delete -n test-ns serviceinstances ups-instance \
  || error_exit 'Error when deleting ups-instance.'

# Delete the broker

echo 'Deleting broker...'

kubectl --context=service-catalog delete servicebrokers ups-broker \
  || error_exit 'Error when deleting ups-broker.'

wait_for_expected_output -x -e 'user-provided-service' \
    kubectl --context=service-catalog get serviceclasses \
  || {
    kubectl --context=service-catalog get serviceclasses
    error_exit 'Service classes not successfully removed upon deleting ups-broker.'
  }

echo 'Walkthrough completed successfully.'
