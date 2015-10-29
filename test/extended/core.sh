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
os::log::install_errexit
cd "${OS_ROOT}"

# ensure_ginkgo_or_die

os::build::setup_env
#if [[ -z ${TEST_ONLY+x} ]]; then
#  go test -c ./test/extended -o ${OS_OUTPUT_BINPATH}/extended.test
#fi

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/core"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
export KUBE_REPO_ROOT="${OS_ROOT}/../../../k8s.io/kubernetes"

function join { local IFS="$1"; shift; echo "$*"; }

# The following skip rules excludes upstream e2e tests that fail.
# TODO: add all users to privileged
SKIP_TESTS=(
  "\[Skipped\]"           # Explicitly skipped upstream

  # Depends on external components, may not need yet
  Monitoring              # Not installed, should be
  "Cluster level logging" # Not installed yet
  Kibana                  # Not installed
  DNS                     # Can't depend on kube-dns
  kube-ui                 # Not installed by default
  DaemonRestart           # Experimental mode not enabled yet
  "Daemon set"            # Experimental mode not enabled yet
  Job                     # Not enabled yet
  "deployment should"     # Not enabled yet
  Ingress                 # Not enabled yet

  # Need fixing
  "Cluster upgrade"       # panic because createNS not called, refactor framework?
  PersistentVolume        # Not skipping on non GCE environments?
  EmptyDir                # TRIAGE
  Proxy                   # TRIAGE
  "Examples e2e"          # TRIAGE: Some are failing due to permissions
  Kubectl                 # TRIAGE: we don't support the kubeconfig flag, and images won't run
  Namespaces              # Namespace controller broken, issue #4731
  "hostPath"              # Need to add ability for the test case to use to hostPath
  "mount an API token into pods" # We add 6 secrets, not 1
  "create a functioning NodePort service" # Tries to bind to port 80, needs cap netsys upstream
  "Networking should function for intra-pod" # Needs two nodes, add equiv test for 1 node, then use networking suite
  "environment variables for services" # Tries to proxy directly to the node, but the underlying cert is wrong?  Is proxy broken?
  "should provide labels and annotations files" # the image can't read the files
  "Ask kubelet to report container resource usage" # container resource usage not exposed yet?
  "should provide Internet connection for containers" # DNS inside container failing!!!
  "able to delete 10 pods per node" # Panic because stats port isn't exposed
  "Kubelet regular resource usage tracking over" # takes too long
  "Kubelet experimental resource usage tracking" # takes too long
  "Resource usage of system containers" # panics in computing resources

  "authentication: OpenLDAP" # needs separate setup and bucketing for openldap bootstrapping

  # Needs triage to determine why it is failing
  "Addon update"          # TRIAGE
  SSH                     # TRIAGE
  Probing                 # TRIAGE
  "should call prestop" # Needs triage, auth maybe
  "be restarted with a /healthz" # used to be working
  "Port forwarding With a server that expects" # used to be working
)
DEFAULT_SKIP=$(join '|' "${SKIP_TESTS[@]}")
SKIP="${SKIP:-$DEFAULT_SKIP}"

if [[ -z ${TEST_ONLY+x} ]]; then
  ensure_iptables_or_die

  function cleanup()
  {
    out=$?
    cleanup_openshift
    echo "[INFO] Exiting"
    exit $out
  }

  trap "exit" INT TERM
  trap "cleanup" EXIT
  echo "[INFO] Starting server"

  setup_env_vars
  reset_tmp_dir
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
fi

echo "[INFO] Running extended tests"

# Run the tests
TMPDIR=${BASETMPDIR} go test -timeout 6h ./test/extended/ --test.v "--ginkgo.skip=${SKIP}" "$@"
