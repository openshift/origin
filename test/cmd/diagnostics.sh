#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit
trap os::test::junit::reconcile_output EXIT

# This test validates the diagnostics command

# available diagnostics (2016-02-24):
# AnalyzeLogs ClusterRegistry ClusterRoleBindings ClusterRoles ClusterRouter ConfigContexts DiagnosticPod MasterConfigCheck MasterNode NodeConfigCheck NodeDefinitions UnitStatus
# Without things feeding into systemd, AnalyzeLogs and UnitStatus are irrelevant.
# The rest should be included in some fashion.

os::test::junit::declare_suite_start "cmd/diagnostics"
os::cmd::expect_success 'oadm diagnostics ClusterRoleBindings ClusterRoles ConfigContexts '
# DiagnosticPod can't run without Docker, would just time out. Exercise flags instead.
os::cmd::expect_success "oadm diagnostics DiagnosticPod --prevent-modification --images=foo"
os::cmd::expect_success "oadm diagnostics MasterConfigCheck NodeConfigCheck --master-config=${MASTER_CONFIG_DIR}/master-config.yaml --node-config=${NODE_CONFIG_DIR}/node-config.yaml"
os::cmd::expect_success_and_text 'oadm diagnostics ClusterRegistry' "DClu1002 from diagnostic ClusterRegistry"
# inexplicably breaks in test env - disable for now
#os::cmd::expect_success_and_text 'oadm diagnostics MasterNode'  'Network plugin does not require master to also run node'
# ClusterRouter fails differently depending on whether other tests have run first, so don't test for specific error
# no ordering allowed
#os::cmd::expect_failure 'oadm diagnostics ClusterRouter' # "DClu2001 from diagnostic ClusterRouter"
os::cmd::expect_failure 'oadm diagnostics NodeDefinitions'
os::cmd::expect_failure_and_text 'oadm diagnostics FakeDiagnostic AlsoMissing' 'No requested diagnostics are available: requested=FakeDiagnostic AlsoMissing'
os::cmd::expect_failure_and_text 'oadm diagnostics AnalyzeLogs AlsoMissing' 'Not all requested diagnostics are available: missing=AlsoMissing requested=AnalyzeLogs AlsoMissing available='

# openshift ex diagnostics is deprecated but not removed. Make sure it works until we consciously remove it.
os::cmd::expect_success 'openshift ex diagnostics ClusterRoleBindings ClusterRoles ConfigContexts '
echo "diagnostics: ok"
os::test::junit::declare_suite_end