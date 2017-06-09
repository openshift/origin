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
    --with-tpr)         WITH_TPR=1 ;;
    --cleanup)          CLEANUP=1 ;;
    --create-artifacts) CREATE_ARTIFACTS=1 ;;
    --fix-auth)         FIX_CONFIGMAP=1 ;;
    *) error_exit "Unrecognized command line parameter: $1" ;;
  esac
  shift
done

CATALOG_RELEASE="catalog"

K8S_KUBECONFIG="${KUBECONFIG:-"~/.kube/config"}"
SC_KUBECONFIG="/tmp/sc-kubeconfig"

function cleanup() {
  export KUBECONFIG="${K8S_KUBECONFIG}"

  if [[ -n "${CREATE_ARTIFACTS:-}" ]]; then
    echo 'Creating artifacts...'
    PREFIX="e2e.test_"
    if [[ -n "${WITH_TPR:-}" ]]; then
      PREFIX+='tpr-backed'
    else
      PREFIX+='etcd-backed'
    fi

    "${ROOT}/contrib/hack/create_artifacts.sh" \
        --prefix "${PREFIX}" --location "${ROOT}" \
        &> /dev/null \
        || true
  fi

  echo 'Cleaning up resources...'
  {
    helm delete --purge "${CATALOG_RELEASE}" || true
    rm -f "${SC_KUBECONFIG}"

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

if [[ -n "${CLEANUP:-}" ]]; then
  trap cleanup EXIT
fi

echo "Running 'e2e.test'..."

# Install catalog
ARGUMENTS="--registry ${REGISTRY}"
ARGUMENTS+=" --version ${VERSION}"
ARGUMENTS+=" --fix-auth"
ARGUMENTS+=" --service-catalog-config ${SC_KUBECONFIG}"
ARGUMENTS+=" --release-name ${CATALOG_RELEASE}"
if [[ -n "${WITH_TPR:-}" ]]; then
  ARGUMENTS+=" --with-tpr"
fi

${ROOT}/contrib/jenkins/install_catalog.sh ${ARGUMENTS} \
  || error_exit "Error installing catalog in cluster."

make bin/e2e.test \
  || error_exit "Error when making e2e test binary."

KUBECONFIG="${KUBECONFIG}" SERVICECATALOGCONFIG="${SC_KUBECONFIG}" ${ROOT}/bin/e2e.test \
  || error_exit "Error while running e2e tests."

echo "'e2e.test' completed successfully."
