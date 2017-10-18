#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

readonly JQSETPULLPOLICY='(.items[] | select(.kind == "DeploymentConfig") | .spec.template.spec.containers[0].imagePullPolicy) |= "IfNotPresent"'

if [[ "${TEST_END_TO_END:-}" != "direct" ]]; then
	if os::util::ensure::system_binary_exists 'docker'; then
		echo "++ Docker is installed, running hack/test-end-to-end-docker.sh instead."
		"${OS_ROOT}/hack/test-end-to-end-docker.sh"
		exit $?
	fi
	echo "++ Docker is not installed, running end-to-end against local binaries"
fi

os::util::ensure::iptables_privileges_exist

os::log::info "Starting end-to-end test"

function cleanup() {
  return_code=$?
  os::test::junit::generate_report
  os::cleanup::all
  os::util::describe_return_code "${return_code}"
  exit "${return_code}"
}
trap "cleanup" EXIT

# Start All-in-one server and wait for health
os::util::environment::use_sudo
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars

os::log::system::start

os::start::configure_server
os::start::server

# set our default KUBECONFIG location
export KUBECONFIG="${ADMIN_KUBECONFIG}"

os::test::junit::declare_suite_start "end-to-end/startup"
if [[ -n "${USE_IMAGES:-}" ]]; then
    os::cmd::expect_success "oc adm registry --dry-run -o json --images='$USE_IMAGES' | jq '$JQSETPULLPOLICY' | oc create -f -"
else
    os::cmd::expect_success "oc adm registry"
fi
os::cmd::expect_success 'oc adm policy add-scc-to-user hostnetwork -z router'
os::cmd::expect_success 'oc adm router'
os::test::junit::declare_suite_end

${OS_ROOT}/test/end-to-end/core.sh
