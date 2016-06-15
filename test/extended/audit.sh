#!/bin/bash
#
# Runs the conformance extended tests for OpenShift with audit enabled

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/test/extended/setup.sh"
cd "${OS_ROOT}"

os::test::extended::setup
# enable auditing
cp ${MASTER_CONFIG_DIR}/master-config.yaml ${MASTER_CONFIG_DIR}/master-config.orig.yaml
openshift ex config patch ${MASTER_CONFIG_DIR}/master-config.orig.yaml \
  --patch="{\"auditConfig\": {\"enabled\": true}}" \
  > ${MASTER_CONFIG_DIR}/master-config.yaml
os::test::extended::start_server
os::test::extended::focus "$@"
os::test::extended::conformance
