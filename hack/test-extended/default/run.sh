#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will start all 'default_*_test.go' test cases.

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
cd "${OS_ROOT}"

source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh

require_ginkgo_or_die

compile_extended

require_iptables_or_die

dirs_image_env_setup_extended

setup_env_vars

trap "exit" INT TERM
trap "cleanup_extended" EXIT

info_msgs_ip_host_setup_extended

configure_os_server

auth_setup_extended

start_os_server

install_router_extended

install_registry_extended

create_image_streams_extended

run_extended_tests "${FOCUS:-"-focus=default:"}"
