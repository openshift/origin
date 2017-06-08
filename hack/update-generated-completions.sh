#!/bin/bash

# This script sets up a go workspace locally and generates shell auto-completion scripts.

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

platform="$(os::build::host_platform)"
if [[ "${platform}" != "linux/amd64" ]]; then
  os::log::warning "Generating completions on ${platform} may not be identical to running on linux/amd64 due to conditional compilation."
fi

OUTPUT_REL_DIR=${1:-""}
OUTPUT_DIR_ROOT="${OS_ROOT}/${OUTPUT_REL_DIR}/contrib/completions"

mkdir -p "${OUTPUT_DIR_ROOT}/bash" || echo $? > /dev/null
mkdir -p "${OUTPUT_DIR_ROOT}/zsh" || echo $? > /dev/null

os::build::gen-completions "${OUTPUT_DIR_ROOT}/bash" "bash"
os::build::gen-completions "${OUTPUT_DIR_ROOT}/zsh" "zsh"