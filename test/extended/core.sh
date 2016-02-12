#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will run all tests that are imported into test/extended.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/lib/log.sh"
os::log::install_errexit

cd "${OS_ROOT}"

# build binaries
if [[ -z $(os::build::find-binary ginkgo) ]]; then
  hack/build-go.sh Godeps/_workspace/src/github.com/onsi/ginkgo/ginkgo
fi
if [[ -z $(os::build::find-binary extended.test) ]]; then
  hack/build-go.sh test/extended/extended.test
fi
if [[ -z $(os::build::find-binary openshift) ]]; then
  hack/build-go.sh
fi
ginkgo="$(os::build::find-binary ginkgo)"
extendedtest="$(os::build::find-binary extended.test)"

source "${OS_ROOT}/hack/lib/util/environment.sh"
os::util::environment::setup_time_vars

if [[ -z ${TEST_ONLY+x} ]]; then
  ensure_iptables_or_die

  function cleanup()
  {
    out=$?
    cleanup_openshift
    echo "[INFO] Exiting"
    return $out
  }

  trap "exit" INT TERM
  trap "cleanup" EXIT
  echo "[INFO] Starting server"

  os::util::environment::setup_all_server_vars "test-extended/core"
  os::util::environment::use_sudo
  reset_tmp_dir

  os::log::start_system_logger

  # when selinux is enforcing, the volume dir selinux label needs to be
  # svirt_sandbox_file_t
  #
  # TODO: fix the selinux policy to either allow openshift_var_lib_dir_t
  # or to default the volume dir to svirt_sandbox_file_t.
  if selinuxenabled; then
         sudo chcon -t svirt_sandbox_file_t ${VOLUME_DIR}
  fi
  configure_os_server
  start_os_server

  export KUBECONFIG="${ADMIN_KUBECONFIG}"

  install_registry
  wait_for_registry
  CREATE_ROUTER_CERT=1 install_router

  echo "[INFO] Creating image streams"
  oc create -n openshift -f examples/image-streams/image-streams-centos7.json --config="${ADMIN_KUBECONFIG}"
else
  # be sure to set VOLUME_DIR if you are running with TEST_ONLY
  echo "[INFO] Not starting server, VOLUME_DIR=${VOLUME_DIR:-}"
fi

# ensure proper relative directories are set
export TMPDIR=${BASETMPDIR:-/tmp}
export EXTENDED_TEST_PATH="$(pwd)/test/extended"
export KUBE_REPO_ROOT="$(pwd)/Godeps/_workspace/src/k8s.io/kubernetes"

if [[ $# -ne 0 ]]; then
  echo "[INFO] Running custom: $@"
  ${extendedtest} "$@"
  exit $?
fi

function join { local IFS="$1"; shift; echo "$*"; }

# Not run by any suite
excluded_tests=(
  "\[Skipped\]"
  "\[Disruptive\]"
  "\[Slow\]"
  "\[Flaky\]"

  # Depends on external components, may not need yet
  Monitoring              # Not installed, should be
  "Cluster level logging" # Not installed yet
  Kibana                  # Not installed
  DNS                     # Can't depend on kube-dns
  kube-ui                 # Not installed by default
  "^Deployment"           # Not enabled yet
  "paused deployment should be ignored by the controller" # Not enabled yet
  "deployment should create new pods" # Not enabled yet
  Ingress                 # Not enabled yet
  "should proxy to cadvisor" # we don't expose cAdvisor port directly for security reasons
  "Cinder"                # requires an OpenStack cluster

  # Need fixing
  "should provide Internet connection for containers" # Needs recursive DNS
  PersistentVolume           # https://github.com/openshift/origin/pull/6884 for recycler
  "mount an API token into pods" # We add 6 secrets, not 1
  "Networking should function for intra-pod" # Needs two nodes, add equiv test for 1 node, then use networking suite
  "should test kube-proxy"   # needs 2 nodes
  "authentication: OpenLDAP" # needs separate setup and bucketing for openldap bootstrapping
  "ConfigMap"                # needs permissions https://github.com/openshift/origin/issues/7096
  "should support exec through an HTTP proxy" # doesn't work because it requires a) static binary b) linux c) kubectl, https://github.com/openshift/origin/issues/7097
  "NFS"                      # no permissions https://github.com/openshift/origin/pull/6884

  # Needs triage to determine why it is failing
  "Addon update"          # TRIAGE
  SSH                     # TRIAGE
  "\[Feature:Upgrade\]"   # TRIAGE

  # Inordinately slow tests
  "should create and stop a working application"
)
common_exclude=$(join '|' "${excluded_tests[@]}")
parallel_test_exclusions=(
  "${excluded_tests[@]}"

  "Service endpoints latency" # requires low latency
)
parallel_exclude=$(join '|' "${parallel_test_exclusions[@]}")

# print the tests we are skipping  
echo "[INFO] The following tests will not be run:"
TEST_OUTPUT_QUIET=true ${extendedtest} "--ginkgo.skip=${common_exclude}" --ginkgo.dryRun | grep skip | sort
echo

# run parallel tests
nodes="${PARALLEL_NODES:-5}"
echo "[INFO] Running parallel tests N=${nodes}"
${ginkgo} -v "-skip=${parallel_exclude}|\[Serial\]" -p -nodes "${nodes}" ${extendedtest} -- -ginkgo.v -test.timeout 6h

# run tests in serial
echo "[INFO] Running serial tests"
${ginkgo} -v "-skip=${common_exclude}" -focus="\[Serial\]" ${extendedtest} -- -ginkgo.v -test.timeout 2h
