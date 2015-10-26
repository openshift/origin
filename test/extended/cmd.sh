#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# No registry or router is setup.
# It is intended to test cli commands that may require docker and therefore
# cannot be run under Travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
os::log::install_errexit
cd "${OS_ROOT}"

ensure_iptables_or_die

os::build::setup_env

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

oc login -u system:admin -n default

echo "[INFO] Running newapp extended tests"

# ensure a local-only image gets a docker image(not imagestream) reference created.
tmp=$(mktemp -d)
pushd $tmp
cat <<-EOF >> Dockerfile
	FROM scratch
	EXPOSE 80
EOF
docker build -t test/scratchimage .
popd
rm -rf $tmp
[ "$(oc new-app  test/scratchimage~https://github.com/openshift/ruby-hello-world.git --strategy=docker -o yaml |& tr '\n' ' ' | grep -E "from:\s+kind:\s+DockerImage\s+name:\s+test/scratchimage:latest\s+")" ]
docker rmi test/scratchimage
