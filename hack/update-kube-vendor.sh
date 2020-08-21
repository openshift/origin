#!/usr/bin/env bash

# This script simplifies updating the vendoring of k8s.io/kubernetes
# and its staging repos from github.com/openshift/kubernetes (or an
# optional override repo).

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SHA="${1:-}"
if [[ -z "${SHA}" ]]; then
  echo >&2 "usage: $0 <SHA> [github.com/username/kubernetes]"
  exit 1
fi

REPO_REPLACEMENT="${2:-}"
if [[ -z "${REPO_REPLACEMENT}" ]]; then
  REPO_REPLACEMENT="github.com/openshift/kubernetes"
fi

UPSTREAM_REPO="k8s.io/kubernetes"

echo "Updating vendoring for ${UPSTREAM_REPO}"
go mod edit -replace "${UPSTREAM_REPO}=${REPO_REPLACEMENT}@${SHA}"
go mod tidy

echo "Updating vendoring for the staging repos of ${UPSTREAM_REPO}"
TARGET_DEPS="$( grep 'staging/src/k8s.io' go.mod | awk '{print $1}' )"
for TARGET_DEP in ${TARGET_DEPS}; do
  go mod edit -replace "${TARGET_DEP}=${REPO_REPLACEMENT}/staging/src/${TARGET_DEP}@${SHA}"
  go mod tidy
done

go mod tidy
go mod vendor
