#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'genopenapi'

ORIGIN_PREFIX="${OS_GO_PACKAGE}/"

INPUT_DIRS=(
  # kube apis
  $(
    grep --color=never -rl '+k8s:openapi-gen=' vendor/k8s.io/kubernetes | \
    xargs -n1 dirname | \
    sed "s,^vendor/,," | \
    sort -u | \
    sed '/^k8s\.io\/kubernetes$/d' | \
    sed '/^k8s\.io\/kubernetes\/staging$/d' | \
    sed 's,k8s\.io/kubernetes/staging/src/,,'
  )

  # origin apis
  $(
    grep --color=never -rl '+k8s:openapi-gen=' pkg | \
    xargs -n1 dirname | \
    sed "s,^,${ORIGIN_PREFIX}," | \
    sort -u
  )
)

INPUT_DIRS=$(IFS=,; echo "${INPUT_DIRS[*]}")

genopenapi \
  --logtostderr \
  --output-base="${GOPATH}/src" \
  --input-dirs "${INPUT_DIRS}" \
  --output-package "${ORIGIN_PREFIX}pkg/openapi" \
  "$@"
