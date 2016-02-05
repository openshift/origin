#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates the diagnostics command

# available diagnostics (2015-12-21):
# AnalyzeLogs ClusterRegistry ClusterRoleBindings ClusterRoles ClusterRouter ConfigContexts DiagnosticPod MasterConfigCheck MasterNode NodeConfigCheck NodeDefinitions UnitStatus
# Without things feeding into systemd, AnalyzeLogs and UnitStatus are irrelevant.
# The rest should be included in some fashion.

os::cmd::expect_success 'openshift ex diagnostics ClusterRoleBindings ClusterRoles ConfigContexts '
# DiagnosticPod can't run without Docker, would just time out. Exercise flags instead.
os::cmd::expect_success "openshift ex diagnostics DiagnosticPod --prevent-modification --images=foo"
os::cmd::expect_success "openshift ex diagnostics MasterConfigCheck NodeConfigCheck --master-config=${MASTER_CONFIG_DIR}/master-config.yaml --node-config=${NODE_CONFIG_DIR}/node-config.yaml"
os::cmd::expect_success_and_text 'openshift ex diagnostics ClusterRegistry' "DClu1002 from diagnostic ClusterRegistry"
# ClusterRouter fails differently depending on whether other tests have run first, so don't test for specific error
# no ordering allowed
#os::cmd::expect_failure 'openshift ex diagnostics ClusterRouter' # "DClu2001 from diagnostic ClusterRouter"
os::cmd::expect_failure 'openshift ex diagnostics NodeDefinitions'
os::cmd::expect_failure_and_text 'openshift ex diagnostics FakeDiagnostic AlsoMissing' 'No requested diagnostics are available: requested=FakeDiagnostic AlsoMissing'
os::cmd::expect_failure_and_text 'openshift ex diagnostics AnalyzeLogs AlsoMissing' 'Not all requested diagnostics are available: missing=AlsoMissing requested=AnalyzeLogs AlsoMissing available='
echo "diagnostics: ok"
