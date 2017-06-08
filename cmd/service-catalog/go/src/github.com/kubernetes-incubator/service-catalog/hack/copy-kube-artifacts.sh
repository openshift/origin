#!/usr/bin/env bash

# this will allow matching files also in subdirs with **/*.json pattern
shopt -s globstar

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

KUBE_ROOT=${1:-""}
KUBE_GODEP_ROOT="${OS_ROOT}/vendor/k8s.io/kubernetes"

if [ -z "$KUBE_ROOT" ]; then
  echo "usage: copy-kube-artifacts.sh <kubernetes root dir>"
  exit 255
fi

# Copy special files.
rsync -av \
  --exclude='BUILD' \
  --include-from=- \
  --include='*/' \
  --exclude='*' \
  --prune-empty-dirs \
  $KUBE_ROOT/ $KUBE_GODEP_ROOT <<EOF
/api/swagger-spec/*.json
/examples/***
/test/e2e/***
/test/fixtures/***
/test/integration/***
/third_party/protobuf/***
/README.md
EOF

rsync -av \
  --exclude='BUILD' \
  --exclude='OWNERS' \
  --exclude='*.go' \
  --include-from=- \
  --include='*/' \
  --exclude='*' \
  --prune-empty-dirs \
  $KUBE_ROOT/ $KUBE_GODEP_ROOT <<EOF
/pkg/***
/plugin/***
/staging/***
EOF
