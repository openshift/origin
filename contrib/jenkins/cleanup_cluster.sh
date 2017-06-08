#!/bin/bash
# Copyright 2016 The Kubernetes Authors.
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

. "${ROOT}/contrib/jenkins/cluster_utilities.sh" || { echo 'Cannot load cluster utilities'; exit 1; }
. "${ROOT}/contrib/hack/utilities.sh" || { echo 'Cannot load Bash utilities'; exit 1; }

while [[ $# -ne 0 ]]; do
  case "$1" in
    --project)    PROJECT="$2" ; shift ;;
    --zone)       ZONE="$2" ; shift ;;
    --kubeconfig) CONFIG="$2" ; shift ;;
    *)            CLUSTERNAME="$1" ;;
  esac
  shift
done

DELETECONFIG='NO'

if [[ -n "${CONFIG:-}" ]]; then
  CONTEXT="$(KUBECONFIG="${CONFIG}" kubectl config current-context)"

  CLUSTERNAME="$(echo "${CONTEXT}" | sed 's/.*_\(.*\)/\1/')"
  ZONE="$(echo "${CONTEXT}" | sed 's/.*_\(.*\)_.*/\1/')"
  PROJECT="$(echo "${CONTEXT}" | sed 's/.*_\(.*\)_.*_.*/\1/')"
else
  [[ -n "${CLUSTERNAME:-}" ]] && [[ -n "${PROJECT:-}" ]] && [[ -n "${ZONE:-}" ]] \
    || error_exit 'Either kubeconfig or cluster, project, and zone must be provided.'

  CONFIG='/tmp/kubeconfig'
  KUBECONFIG="${CONFIG}" gcloud container clusters get-credentials "${CLUSTERNAME}" \
      --project "${PROJECT}" --zone "${ZONE}" \
    || error_exit "Could not get credentials for cluster ${CLUSTERNAME} in project ${PROJECT}."

  DELETECONFIG='YES'
fi

gcloud container clusters delete "${CLUSTERNAME}" --project="${PROJECT}" \
    --zone="${ZONE}" --quiet --async \
  || error_exit 'Failed to delete cluster on Google Cloud Platform.'

if [[ "${DELETECONFIG}" == 'YES' ]]; then
  rm "${CONFIG}"
fi
