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
GOPATH="${GOPATH:-${ROOT%/src/github.com/kubernetes-incubator/service-catalog}}"

. "${ROOT}/contrib/hack/utilities.sh" || { echo 'Cannot load bash utilities.'; exit 1; }

# Parse command line arguments.
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-docker-compile) NO_DOCKER_COMPILE=yes ;;
    --project)           PROJECT="${2:-}" ; shift ;;
    --version)           VERSION="${2:-}" ; shift ;;
    --coverage)          COVERAGE="${2:-}"; shift ;;

    *) error_exit "Unrecognized command line flag $1" ;;
  esac
  shift
done

[[ -n "${PROJECT:-}" ]] \
  || error_exit '--project is a required parameter'

if [[ "$(uname -s)" == "Linux" ]]; then
  MAKE_VARS=(
    V=1
    VERSION="${VERSION:-"canary"}"
  )

  [[ -n "${NO_DOCKER_COMPILE:-}" ]] && MAKE_VARS+=(NO_DOCKER=1)

  make "${MAKE_VARS[@]}" build \
    || error_exit 'build linux failed.'

  make "${MAKE_VARS[@]}" test \
    || error_exit 'test linux failed.'

  make "${MAKE_VARS[@]}" images \
    || error_exit 'images linux failed.'

  gcloud docker --authorize-only --server=gcr.io \
    || error_exit 'gcloud docker authorization failed'

  make "${MAKE_VARS[@]}" REGISTRY=gcr.io/${PROJECT}/catalog/ push \
    || error_exit 'make push failed.'

  docker images \
    || error_exit 'Cannot run docker images.'
fi

echo 'build.sh completed successfully'
