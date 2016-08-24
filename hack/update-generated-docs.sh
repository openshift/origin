#!/bin/bash

# This script sets up a go workspace locally and generates the documents and manuals.

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

"${OS_ROOT}/hack/build-go.sh" tools/gendocs tools/genman

# Find binary
gendocs="$(os::build::find-binary gendocs)"
genman="$(os::build::find-binary genman)"

if [[ -z "$gendocs" ]]; then
  {
    echo "It looks as if you don't have a compiled gendocs binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh tools/gendocs'."
  } >&2
  exit 1
fi

if [[ -z "$genman" ]]; then
  {
    echo "It looks as if you don't have a compiled genman binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh tools/genman'"
  } >&2
  exit 1
fi

OUTPUT_DIR_REL=${1:-""}
OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/generated"
MAN_OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/man/man1"

mkdir -p "${OUTPUT_DIR}" || echo $? > /dev/null
mkdir -p "${MAN_OUTPUT_DIR}" || echo $? > /dev/null

os::build::gen-docs "${gendocs}" "${OUTPUT_DIR}"
os::build::gen-man "${genman}" "${MAN_OUTPUT_DIR}" "oc"
os::build::gen-man "${genman}" "${MAN_OUTPUT_DIR}" "openshift"
os::build::gen-man "${genman}" "${MAN_OUTPUT_DIR}" "oadm"