#!/bin/bash

# This script sets up a go workspace locally and generates the documents and manuals.

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::built_binary_exists 'gendocs'
os::util::ensure::built_binary_exists 'genman'

OUTPUT_DIR_REL=${1:-""}
OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/generated"
MAN_OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/man/man1"

mkdir -p "${OUTPUT_DIR}" || echo $? > /dev/null
mkdir -p "${MAN_OUTPUT_DIR}" || echo $? > /dev/null

os::build::gen-docs "${gendocs}" "${OUTPUT_DIR}"
os::build::gen-man "${genman}" "${MAN_OUTPUT_DIR}" "oc"
os::build::gen-man "${genman}" "${MAN_OUTPUT_DIR}" "openshift"
os::build::gen-man "${genman}" "${MAN_OUTPUT_DIR}" "oadm"