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

ensure_ginkgo_or_die
ensure_iptables_or_die

os::build::setup_env
if [[ -z ${TEST_ONLY+x} ]]; then
	go test -c ./test/extended -o ${OS_OUTPUT_BINPATH}/extended.test
fi

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/authentication"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
export KUBE_REPO_ROOT="${OS_ROOT}/../../../k8s.io/kubernetes"

function join { local IFS="$1"; shift; echo "$*"; }


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
configure_os_server
start_os_server

export KUBECONFIG="${ADMIN_KUBECONFIG}"

install_registry
wait_for_registry

oc login ${MASTER_ADDR} -u ldap -p password --certificate-authority=${MASTER_CONFIG_DIR}/ca.crt
oc new-project openldap

# create all the resources we need
oc create -f test/extended/fixtures/ldap

# wait until the last event that occured on the imagestream was the successful pull of the latest image
wait_for_command 'oc get imagestream openldap --template="{{with \$tags := .status.tags}}{{with \$event := index \$tags 0}}{{\$event.tag}}{{end}}{{end}}" | grep latest' $((60*TIME_SEC))

# kick off a build and wait for it to finish
oc start-build openldap --follow

# wait for the deployment to be up and running
wait_for_command 'oc get pods -l deploymentconfig=openldap-server --template="{{with \$items := .items}}{{with \$item := index \$items 0}}{{\$item.status.phase}}{{end}}{{end}}" | grep Running' $((60*TIME_SEC))

LDAP_SERVICE_IP=$(oc get --output-version=v1beta3 --template="{{ .spec.portalIP }}" service openldap-server)


oc login -u system:admin -n default

echo "[INFO] Running extended tests"

sleep 10

cp test/extended/authentication/sync-schema1-group1.yaml ${BASETMPDIR}
os::util::sed "s/LDAP_SERVICE_IP/${LDAP_SERVICE_IP}/g" ${BASETMPDIR}/sync-schema1-group1.yaml
openshift ex sync-groups --sync-config=${BASETMPDIR}/sync-schema1-group1.yaml --confirm

# Run the tests
#LDAP_IP=${LDAP_SERVICE_IP} TMPDIR=${BASETMPDIR} ginkgo -progress -stream -v -focus="authentication: OpenLDAP" ${OS_OUTPUT_BINPATH}/extended.test
