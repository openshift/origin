#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'genopenapi'

ORIGIN_PREFIX=github.com/openshift/origin

INPUT_DIRS=(
  # kube apis
  $(
    find ./vendor/k8s.io/kubernetes -name \*.go | \
    xargs grep --color=never -l '+k8s:openapi-gen=' | \
    xargs -n1 dirname | \
    sed "s,^\./vendor/,," | \
    sort -u
  )

  # origin apis
  $(
    find . \( -path ./vendor -o -path ./_output \) -prune -type f -o -name \*.go | \
    xargs grep --color=never -l '+k8s:openapi-gen=' | \
    xargs -n1 dirname | \
    sed "s,^\.,${ORIGIN_PREFIX}," | \
    sort -u
  )
)

INPUT_DIRS=$(IFS=,; echo "${INPUT_DIRS[*]}")

genopenapi \
	--logtostderr \
	--output-base="${GOPATH}/src" \
	--input-dirs "${INPUT_DIRS}" \
	--output-package "${ORIGIN_PREFIX}/pkg/openapi" \
	"$@"
