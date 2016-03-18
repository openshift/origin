#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
# this will allow matching files also in subdirs with **/*.json pattern
shopt -s globstar

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

KUBE_ROOT=${1:-""}
KUBE_GODEP_ROOT="${OS_ROOT}/Godeps/_workspace/src/k8s.io/kubernetes"

if [ -z "$KUBE_ROOT" ]; then
  echo "usage: copy-kube-artifacts.sh <kubernetes root dir>"
  exit 255
fi

# Copy special files.
rsync -av \
  --exclude='examples/blog-logging/diagrams/***' \
  --exclude='pkg/ui/data/swagger/datafile.go' \
  --include-from=- \
  --include='*/' \
  --exclude='*' \
  --prune-empty-dirs \
  $KUBE_ROOT/ $KUBE_GODEP_ROOT <<EOF
/api/swagger-spec/*.json
/cmd/integration/***
/cmd/kube-apiserver/***
/cmd/kube-controller-manager/***
/cmd/kube-proxy/***
/cmd/kubectl/***
/cmd/kubelet/***
/cmd/libs/***
/docs/admin/**.json
/docs/admin/**.yaml
/docs/user-guide/**.json
/docs/user-guide/**.yaml
/docs/user-guide/simple-yaml.md
/docs/user-guide/walkthrough/README.md
/examples/***
/pkg/***
/plugin/***
/test/e2e/***
/test/fixtures/***
/test/integration/***
/third_party/golang/***
/README.md
EOF
