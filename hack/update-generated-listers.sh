#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'genlisters'

PREFIX="${OS_GO_PACKAGE}/"

INPUT_DIRS=$(
  grep -rl genclient=true pkg | \
  xargs -n1 dirname | \
  grep -v 'pkg\/security\/api' | \
  sort -u | \
  sed "s,^,${PREFIX}," | \
  paste -sd,
)

genlisters \
  --logtostderr \
  --input-dirs "${INPUT_DIRS}" \
  "$@"
