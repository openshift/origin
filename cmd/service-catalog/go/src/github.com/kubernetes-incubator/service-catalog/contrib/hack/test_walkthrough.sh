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
    --registry)       REGISTRY="${2:-}"; shift ;;
    --version)        VERSION="${2:-}"; shift ;;
    --with-tpr)       WITH_TPR=1 ;;
    --cleanup)        CLEANUP=1 ;;
    --create-artifacts) CREATE_ARTIFACTS=1 ;;
    --fix-auth)   FIX_CONFIGMAP=1 ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

BROKER_RELEASE="ups-broker"
CATALOG_RELEASE="catalog"

K8S_KUBECONFIG="${KUBECONFIG:-~/.kube/config}"
SC_KUBECONFIG="/tmp/sc-kubeconfig"

VERSION="${VERSION:-"canary"}"
REGISTRY="${REGISTRY:-}"
CONTROLLER_MANAGER_IMAGE="${REGISTRY}controller-manager:${VERSION}"
APISERVER_IMAGE="${REGISTRY}apiserver:${VERSION}"
UPS_BROKER_IMAGE="${REGISTRY}user-broker:${VERSION}"

echo 'TESTING WALKTHROUGH'
echo '-------------------'
echo "Using kubeconfig: ${K8S_KUBECONFIG}"
echo "Using service catalog kubeconfig: ${SC_KUBECONFIG}"
echo "Using controller-manager image: ${CONTROLLER_MANAGER_IMAGE}"
echo "Using apiserver image: ${APISERVER_IMAGE}"
echo "Using ups-broker image: ${UPS_BROKER_IMAGE}"
echo '-------------------'

function cleanup() {
  if [[ -n "${CREATE_ARTIFACTS:-}" ]]; then
    echo 'Creating artifacts...'
    PREFIX='walkthrough_'
    if [[ -n "${WITH_TPR:-}" ]]; then
      PREFIX+='tpr-backed'
    else
      PREFIX+='etcd-backed'
    fi

    KUBECONFIG="${K8S_KUBECONFIG}" "${ROOT}/contrib/hack/create_artifacts.sh" \
        --prefix "${PREFIX}" --location "${ROOT}" \
        &> /dev/null \
        || true
  fi

  echo 'Cleaning up resources...'
  {
    export KUBECONFIG="${K8S_KUBECONFIG}"
    helm delete --purge "${BROKER_RELEASE}" || true
    helm delete --purge "${CATALOG_RELEASE}" || true
    rm -f "${SC_KUBECONFIG}"
    kubectl delete secret -n test-ns ups-binding || true
    kubectl delete namespace test-ns || true

    wait_for_expected_output -x -e 'test-ns' -n 10 \
      kubectl get namespaces

    # TODO: Hack in order to delete TPRs. Will need to be removed when TPRs can be deleted
    # by the catalog API server.
    if [[ -n "${WITH_TPR:-}" ]]; then
      kubectl delete thirdpartyresources binding.servicecatalog.k8s.io
      kubectl delete thirdpartyresources instance.servicecatalog.k8s.io
      kubectl delete thirdpartyresources broker.servicecatalog.k8s.io
      kubectl delete thirdpartyresources service-class.servicecatalog.k8s.io
    fi
  } &> /dev/null
}

# Deploying to cluster

if [[ -n "${CLEANUP:-}" ]]; then
  trap cleanup EXIT
fi

echo 'Creating "test-ns" namespace...'

kubectl create namespace test-ns \
  || error_exit 'Error creating "test-ns" namespace.'

echo 'Deploying user-provided-service broker...'

VALUES="image=${UPS_BROKER_IMAGE}"

retry -n 10 \
    helm install "${ROOT}/charts/ups-broker" \
    --name "${BROKER_RELEASE}" \
    --namespace "ups-broker" \
    --set "${VALUES}" \
  || error_exit 'Error deploying ups-broker to cluster.'

echo 'Deploying service catalog...'

retry -n 10 \
    kubectl --namespace kube-system get configmap extension-apiserver-authentication \
  || error_exit 'Timed out waiting for extension-apiserver-authentication configmap to come up.'

# The API server automatically provisions the configmap, but we need it to contain the requestheader CA as well,
# which is only provisioned if you pass the appropriate flags to master.  GKE doesn't do this, so we work around it.
if [[ -n "${FIX_CONFIGMAP:-}" ]] && [[ -z "$(kubectl --namespace kube-system get configmap extension-apiserver-authentication -o jsonpath="{ $.data['requestheader-client-ca-file'] }")" ]]; then
    full_configmap=$(kubectl --namespace kube-system get configmap extension-apiserver-authentication -o json)
    echo "$full_configmap" | jq '.data["requestheader-client-ca-file"] = .data["client-ca-file"]' | kubectl --namespace kube-system update configmap extension-apiserver-authentication -f -
    [[ -n "$(kubectl --namespace kube-system get configmap extension-apiserver-authentication -o jsonpath="{ $.data['requestheader-client-ca-file'] }")" ]] || { echo "Could not add requestheader auth CA to extension-apiserver-authentication configmap."; exit 1; }
fi

VALUES='debug=true'
VALUES+=',insecure=true'
VALUES+=",controllerManager.image=${CONTROLLER_MANAGER_IMAGE}"
VALUES+=",apiserver.image=${APISERVER_IMAGE}"
VALUES+=',apiserver.service.type=LoadBalancer'
if [[ -n "${WITH_TPR:-}" ]]; then
  VALUES+=',apiserver.storage.type=tpr'
  VALUES+=',apiserver.storage.tpr.globalNamespace=test-ns'
fi

retry -n 10 \
    helm install "${ROOT}/charts/catalog" \
    --name "${CATALOG_RELEASE}" \
    --namespace "catalog" \
    --set "${VALUES}" \
  || error_exit 'Error deploying service catalog to cluster.'

# Waiting for everything to come up

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

echo 'Waiting on external IP for service catalog API Server...'

wait_for_expected_output -x -e 'pending' -n 10 \
    kubectl get services --namespace catalog \
  || error_exit 'Timed out waiting for external IP for service catalog API Server.'

# Create kubeconfig for service catalog API server

echo 'Connecting to service catalog API Server...'

API_SERVER_HOST="$(kubectl get services -n catalog | grep 'apiserver' | awk '{print $3}')"

[[ "${API_SERVER_HOST}" =~ ^[0-9.]*$ ]] \
  || error_exit 'Error when fetching service catalog API Server IP address.'

export KUBECONFIG="${SC_KUBECONFIG}"

kubectl config set-credentials service-catalog-creds --username=admin --password=admin
kubectl config set-cluster service-catalog-cluster --server="https://${API_SERVER_HOST}:443" --insecure-skip-tls-verify=true
kubectl config set-context service-catalog-ctx --cluster=service-catalog-cluster --user=service-catalog-creds
kubectl config use-context service-catalog-ctx

retry -n 10 \
  kubectl get brokers,serviceclasses,instances,bindings \
  || error_exit 'Issue listing resources from service catalog API server.'

# Create the broker

echo 'Creating broker...'

kubectl create -f "${ROOT}/contrib/examples/walkthrough/ups-broker.yaml" \
  || error_exit 'Error when creating ups-broker.'

wait_for_expected_output -e 'FetchedCatalog' -n 10 \
    kubectl get brokers ups-broker -o yaml \
  || {
    kubectl get brokers ups-broker -o yaml
    error_exit 'Did not receive expected condition when creating ups-broker.'
  }

[[ "$(kubectl get brokers ups-broker -o yaml)" == *"status: \"True\""* ]] \
  || {
    kubectl get brokers ups-broker -o yaml
    error_exit 'Failure status reported when attempting to fetch catalog from ups-broker.'
  }

[[ "$(kubectl get serviceclasses)" == *user-provided-service* ]] \
  || error_exit 'user-provided-service not listed when fetching service classes.'

# Provision an instance

echo 'Provisioning instance...'

kubectl create -f "${ROOT}/contrib/examples/walkthrough/ups-instance.yaml" \
  || error_exit 'Error when creating ups-instance.'

wait_for_expected_output -e 'ProvisionedSuccessfully' -n 10 \
  kubectl get instances -n test-ns ups-instance -o yaml \
  || {
    kubectl get instances -n test-ns ups-instance -o yaml
    error_exit 'Did not receive expected condition when provisioning ups-instance.'
  }

[[ "$(kubectl get instances -n test-ns ups-instance -o yaml)" == *"status: \"True\""* ]] \
  || {
    kubectl get instances -n test-ns ups-instance -o yaml
    error_exit 'Failure status reported when attempting to provision ups-instance.'
  }

# Bind to the instance

echo 'Binding to instance...'

kubectl create -f "${ROOT}/contrib/examples/walkthrough/ups-binding.yaml" \
  || error_exit 'Error when creating ups-binding.'

wait_for_expected_output -e 'InjectedBindResult' -n 10 \
  kubectl get bindings -n test-ns ups-binding -o yaml \
  || {
    kubectl get bindings -n test-ns ups-binding -o yaml
    error_exit 'Did not receive expected condition when injecting ups-binding.'
  }

[[ "$(kubectl get bindings -n test-ns ups-binding -o yaml)" == *"status: \"True\""* ]] \
  || {
    kubectl get bindings -n test-ns ups-binding -o yaml
    error_exit 'Failure status reported when attempting to inject ups-binding.'
  }

[[ "$(KUBECONFIG="${K8S_KUBECONFIG}" kubectl get secrets -n test-ns)" == *ups-binding* ]] \
  || error_exit '"ups-binding" not present when listing secrets.'


# TODO: Cannot currently test TPR deletion; only delete if using an etcd-backed
# API server
if [[ -z "${WITH_TPR:-}" ]]; then
  # Unbind from the instance

  echo 'Unbinding from instance...'

  kubectl delete -n test-ns bindings ups-binding \
    || error_exit 'Error when deleting ups-binding.'

  export KUBECONFIG="${K8S_KUBECONFIG}"
  wait_for_expected_output -x -e "ups-binding" -n 10 \
      kubectl get secrets -n test-ns \
    || error_exit '"ups-binding" not removed upon deleting ups-binding.'
  export KUBECONFIG="${SC_KUBECONFIG}"

  # Deprovision the instance

  echo 'Deprovisioning instance...'

  kubectl delete -n test-ns instances ups-instance \
    || error_exit 'Error when deleting ups-instance.'

  # Delete the broker

  echo 'Deleting broker...'

  kubectl delete brokers ups-broker \
    || error_exit 'Error when deleting ups-broker.'

  wait_for_expected_output -x -e 'user-provided-service' -n 10 \
      kubectl get serviceclasses \
    || {
      kubectl get serviceclasses
      error_exit 'Service classes not successfully removed upon deleting ups-broker.'
    }
fi

echo 'Walkthrough completed successfully.'
