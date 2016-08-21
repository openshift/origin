#!/usr/bin/env bash

# this will allow matching files also in subdirs with **/*.json pattern
shopt -s globstar

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

GODEP_ROOT="${OS_ROOT}/vendor"
KUBE_ROOT=${1:-""}
KUBE_GODEP_ROOT="${GODEP_ROOT}/k8s.io/kubernetes"

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
/federation/client/clientset_generated/**.go
/pkg/***
/plugin/***
/test/e2e/***
/test/fixtures/***
/test/integration/***
/third_party/golang/***
/third_party/protobuf/***
/README.md
EOF

# Copy extra vendored files that aren't direct dependencies of any package
rsync -av \
  --exclude='examples/blog-logging/diagrams/***' \
  --exclude='pkg/ui/data/swagger/datafile.go' \
  --include-from=- \
  --include='*/' \
  --exclude='*' \
  --prune-empty-dirs \
  $KUBE_ROOT/vendor/ $GODEP_ROOT <<EOF
/github.com/onsi/ginkgo/ginkgo/**.go
/github.com/golang/mock/gomock/**.go
/github.com/google/cadvisor/info/v1/test/**.go
EOF
