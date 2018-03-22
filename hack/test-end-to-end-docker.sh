#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Starting containerized end-to-end test"

unset KUBECONFIG

os::util::environment::use_sudo
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars
os::util::environment::setup_time_vars
export HOME="${FAKE_HOME_DIR}"

function cleanup() {
	return_code=$?

	os::test::junit::generate_report
	os::cleanup::all

	# restore journald to previous form
	if os::util::ensure::system_binary_exists 'systemctl'; then
		os::log::info "Restoring journald limits"
		${USE_SUDO:+sudo} mv /etc/systemd/{journald.conf.bak,journald.conf}
		${USE_SUDO:+sudo} systemctl restart systemd-journald.service
		# Docker has "some" problems when journald is restarted, so we need to
		# restart docker, as well.
		${USE_SUDO:+sudo} systemctl restart docker.service
	fi

	os::util::describe_return_code "${return_code}"
	exit "${return_code}"
}
trap "cleanup" EXIT

os::log::system::start

# This turns-off rate limiting in journald to bypass the problem from
# https://github.com/openshift/origin/issues/12558.
if os::util::ensure::system_binary_exists 'systemctl'; then
	os::log::info "Turning off journald limits"
	${USE_SUDO:+sudo} cp /etc/systemd/{journald.conf,journald.conf.bak}
	os::util::sed "s/^.*RateLimitInterval.*$/RateLimitInterval=0/g" /etc/systemd/journald.conf
	os::util::sed "s/^.*RateLimitBurst.*$/RateLimitBurst=0/g" /etc/systemd/journald.conf
	${USE_SUDO:+sudo} systemctl restart systemd-journald.service
	# Docker has "some" problems when journald is restarted, so we need to
	# restart docker, as well.
	${USE_SUDO:+sudo} systemctl restart docker.service
fi

# Tag the web console image with the same tag as the other origin images
docker pull openshift/origin-web-console:latest
docker tag openshift/origin-web-console:latest openshift/origin-web-console:${TAG}

# Setup
os::log::info "openshift version: `openshift version`"
os::log::info "oc version:        `oc version`"
os::log::info "Using images:							${USE_IMAGES}"

os::log::info "Starting OpenShift containerized server"
CLUSTERUP_DIR="${BASETMPDIR}"/cluster-up
mkdir "${CLUSTERUP_DIR}"
oc cluster up --server-loglevel=4 --tag="${TAG}" \
        --base-dir="${CLUSTERUP_DIR}" \
        --write-config
        
oc cluster up --server-loglevel=4 --tag="${TAG}" \
        --base-dir="${CLUSTERUP_DIR}"

MASTER_CONFIG_DIR="${CLUSTERUP_DIR}/oc-cluster-up-kube-apiserver/master"

os::test::junit::declare_suite_start "setup/start-oc_cluster_up"
os::cmd::try_until_success "oc cluster status" "$((5*TIME_MIN))" "10"
os::test::junit::declare_suite_end

IMAGE_WORKING_DIR=/var/lib/origin
docker cp origin:${IMAGE_WORKING_DIR}/openshift.local.config ${BASETMPDIR}

export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
export KUBECONFIG="${ADMIN_KUBECONFIG}"
os::log::info "To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"


${OS_ROOT}/test/end-to-end/core.sh
