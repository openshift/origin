#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

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
  --include-from=- \
  --exclude='*' \
  $KUBE_ROOT/ $KUBE_GODEP_ROOT <<EOF
api/swagger-spec/v1.json
cmd/integration/**.json
cmd/integration/**.yaml
docs/admin/**.json
docs/admin/**.yaml
docs/user-guide/**.json
docs/user-guide/**.yaml
docs/user-guide/simple-yaml.md
docs/user-guide/walkthrough/README.md
examples/***
pkg/client/testdata/myCA.cer
pkg/client/testdata/myCA.key
pkg/client/testdata/mycertvalid.cer
pkg/client/testdata/mycertvalid.key
pkg/client/testdata/mycertvalid.req
README.md
EOF
