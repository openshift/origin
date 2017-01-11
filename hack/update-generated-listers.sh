#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'genlisters'

PREFIX=github.com/openshift/origin

INPUT_DIRS=$(
  find . -not  \( \( -wholename '*/vendor/*' \) -prune \) -name '*.go' | \
	xargs grep --color=never -l '+k8s:defaulter-gen=' | \
	xargs -n1 dirname | \
	sed "s,^\.,${PREFIX}," | \
	sort -u | \
	paste -sd,
)

INPUT_DIRS=(
  $(
    cd ${OS_ROOT}
    find pkg -name \*.go | \
      xargs grep -l genclient=true | \
      xargs -n1 dirname | \
      sort -u | \
      grep -v 'pkg\/security\/api'
  )
)

INPUT_DIRS=(${INPUT_DIRS[@]/#/github.com/openshift/origin/})
INPUT_DIRS=$(IFS=,; echo "${INPUT_DIRS[*]}")


genlisters \
	--logtostderr \
	--input-dirs ${INPUT_DIRS[@]} \
	"$@"
