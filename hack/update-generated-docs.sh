#!/bin/bash

# This script sets up a go workspace locally and generates the documents and manuals.

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

OUTPUT_DIR_REL=${1:-""}
OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/generated"
MAN_OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/man/man1"

# Generate fresh docs
os::util::gen-docs ${1:-""}

# Replace with placeholder docs
os::util::set-docs-placeholder "${OUTPUT_DIR}"
os::util::set-man-placeholder "${MAN_OUTPUT_DIR}"
