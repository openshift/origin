#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

os::log::install_errexit

function cleanup()
{
    out=$?
    pkill -P $$

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
    else
        echo
        echo "Complete"
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

# Prevent user environment from colliding with the test setup
unset KUBECONFIG
unset OPENSHIFTCONFIG

USE_LOCAL_IMAGES=${USE_LOCAL_IMAGES:-true}

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_SCHEME=${API_SCHEME:-https}
API_PORT=${API_PORT:-8443}
API_HOST=${API_HOST:-127.0.0.1}
MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
KUBELET_SCHEME=${KUBELET_SCHEME:-https}
KUBELET_HOST=${KUBELET_HOST:-127.0.0.1}
KUBELET_PORT=${KUBELET_PORT:-10250}

TEMP_DIR=${USE_TEMP:-$(mktemp -d /tmp/openshift-cmd.XXXX)}
ETCD_DATA_DIR="${TEMP_DIR}/etcd"
VOLUME_DIR="${TEMP_DIR}/volumes"
CERT_DIR="${TEMP_DIR}/certs"
CONFIG_DIR="${TEMP_DIR}/configs"
mkdir -p "${ETCD_DATA_DIR}" "${VOLUME_DIR}" "${CERT_DIR}" "${CONFIG_DIR}"

# handle profiling defaults
profile="${OPENSHIFT_PROFILE-}"
unset OPENSHIFT_PROFILE
if [[ -n "${profile}" ]]; then
    if [[ "${TEST_PROFILE-}" == "cli" ]]; then
        export CLI_PROFILE="${profile}"
    else
        export WEB_PROFILE="${profile}"
    fi
fi

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

# Check openshift version
out=$(openshift version)
echo openshift: $out

# profile the web
export OPENSHIFT_PROFILE="${WEB_PROFILE-}"

# Specify the scheme and port for the listen address, but let the IP auto-discover. Set --public-master to localhost, for a stable link to the console.
echo "[INFO] Create certificates for the OpenShift server to ${CERT_DIR}"
# find the same IP that openshift start will bind to.  This allows access from pods that have to talk back to master
ALL_IP_ADDRESSES=`ifconfig | grep "inet " | awk '{print $2}'`
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost"
while read -r IP_ADDRESS
do
    SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
done <<< "${ALL_IP_ADDRESSES}"

openshift admin create-master-certs \
  --overwrite=false \
  --cert-dir="${CERT_DIR}" \
  --hostnames="${SERVER_HOSTNAME_LIST}" \
  --master="${MASTER_ADDR}" \
  --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}"

openshift admin create-node-config \
  --listen="${KUBELET_SCHEME}://0.0.0.0:${KUBELET_PORT}" \
  --node-dir="${CERT_DIR}/node-${KUBELET_HOST}" \
  --node="${KUBELET_HOST}" \
  --hostnames="${KUBELET_HOST}" \
  --master="${MASTER_ADDR}" \
  --node-client-certificate-authority="${CERT_DIR}/ca/cert.crt" \
  --certificate-authority="${CERT_DIR}/ca/cert.crt" \
  --signer-cert="${CERT_DIR}/ca/cert.crt" \
  --signer-key="${CERT_DIR}/ca/key.key" \
  --signer-serial="${CERT_DIR}/ca/serial.txt"

# Start openshift
OPENSHIFT_ON_PANIC=crash openshift start \
  --master="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --hostname="${KUBELET_HOST}" \
  --volume-dir="${VOLUME_DIR}" \
  --cert-dir="${CERT_DIR}" \
  --etcd-dir="${ETCD_DATA_DIR}" \
  --create-certs=false 1>&2 &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
    export CURL_CA_BUNDLE="${CERT_DIR}/ca/cert.crt"
    export CURL_CERT="${CERT_DIR}/admin/cert.crt"
    export CURL_KEY="${CERT_DIR}/admin/key.key"
fi

# set the home directory so we don't pick up the users .config
export HOME="${CERT_DIR}/admin"

wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "kubelet: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta1/minions/${KUBELET_HOST}" "apiserver(minions): " 0.25 80

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

# test client not configured
[ "$(osc get services 2>&1 | grep 'no server found')" ]

# Set KUBERNETES_MASTER for osc from now on
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Set certificates for osc from now on
if [[ "${API_SCHEME}" == "https" ]]; then
    # test bad certificate
    [ "$(osc get services 2>&1 | grep 'certificate signed by unknown authority')" ]

    # ignore anything in the running user's $HOME dir
    export HOME="${CERT_DIR}/admin"
fi

# test config files from the --config flag
osc get services --config="${CERT_DIR}/admin/.kubeconfig"

# test config files from env vars
OPENSHIFTCONFIG="${CERT_DIR}/admin/.kubeconfig" osc get services
KUBECONFIG="${CERT_DIR}/admin/.kubeconfig" osc get services

# test config files in the current directory
TEMP_PWD=`pwd` 
pushd ${CONFIG_DIR} >/dev/null
    cp ${CERT_DIR}/admin/.kubeconfig .openshiftconfig
    ${TEMP_PWD}/${GO_OUT}/osc get services
    mv .openshiftconfig .kubeconfig 
    ${TEMP_PWD}/${GO_OUT}/osc get services
popd 

# test config files in the home directory
mv ${CONFIG_DIR} ${HOME}/.kube
osc get services
mkdir -p ${HOME}/.config
mv ${HOME}/.kube ${HOME}/.config/openshift
mv ${HOME}/.config/openshift/.kubeconfig ${HOME}/.config/openshift/.config
osc get services
echo "config files: ok"
export OPENSHIFTCONFIG="${HOME}/.config/openshift/.config"

# from this point every command will use config from the OPENSHIFTCONFIG env var

osc get templates
osc create -f examples/sample-app/application-template-dockerbuild.json
osc get templates
osc get templates ruby-helloworld-sample
osc process ruby-helloworld-sample
osc describe templates ruby-helloworld-sample
osc delete templates ruby-helloworld-sample
osc get templates
# TODO: create directly from template
echo "templates: ok"

# verify some default commands
[ "$(openshift cli)" ]
[ "$(openshift ex)" ]
[ "$(openshift admin config 2>&1)" ]
[ "$(openshift cli config 2>&1)" ]
[ "$(openshift ex tokens)" ]
[ "$(openshift admin policy  2>&1)" ]
[ "$(openshift kubectl 2>&1)" ]
[ "$(openshift kube 2>&1)" ]
[ "$(openshift admin 2>&1)" ]

# help for root commands must be consistent
[ "$(openshift | grep 'OpenShift Application Platform')" ]
[ "$(osc | grep 'OpenShift Client')" ]
[ "! $(osc | grep 'Options')" ]
[ "! $(osc | grep 'Global Options')" ]
[ "$(openshift cli | grep 'OpenShift Client')" ]
[ "$(openshift kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(osadm 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift admin 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for root commands with --help flag must be consistent
[ "$(openshift --help 2>&1 | grep 'OpenShift Application Platform')" ]
[ "$(osc --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(osc login --help 2>&1 | grep 'Options')" ]
[ "! $(osc login --help 2>&1 | grep 'Global Options')" ]
[ "$(openshift cli --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift kubectl --help 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(osadm --help 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift admin --help 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for root commands through help command must be consistent
[ "$(openshift help cli 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift help kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(openshift help admin 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for given command with --help flag must be consistent
[ "$(osc get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift start --help 2>&1 | grep 'Start an OpenShift all-in-one server')" ]
[ "$(openshift start master --help 2>&1 | grep 'Start an OpenShift master')" ]
[ "$(openshift start node --help 2>&1 | grep 'Start an OpenShift node')" ]
[ "$(osc get --help 2>&1 | grep 'osc')" ]

# help for given command through help command must be consistent
[ "$(osc help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help start 2>&1 | grep 'Start an OpenShift all-in-one server')" ]
[ "$(openshift help start master 2>&1 | grep 'Start an OpenShift master')" ]
[ "$(openshift help start node 2>&1 | grep 'Start an OpenShift node')" ]
[ "$(openshift cli help update 2>&1 | grep 'openshift')" ]

# runnable commands with required flags must error consistently
[ "$(osc get 2>&1 | grep 'you must provide one or more resources')" ]
[ "$(openshift cli get 2>&1 | grep 'you must provide one or more resources')" ]
[ "$(openshift kubectl get 2>&1 | grep 'you must provide one or more resources')" ]

osc get pods --match-server-version
osc create -f examples/hello-openshift/hello-pod.json
osc describe pod hello-openshift
osc delete pods hello-openshift
echo "pods: ok"

osc get services
osc create -f test/integration/fixtures/test-service.json
osc delete services frontend
echo "services: ok"

osc get minions
echo "minions: ok"

osc get images
osc create -f test/integration/fixtures/test-image.json
osc delete images test
echo "images: ok"

osc get imageStreams
osc create -f test/integration/fixtures/test-image-stream.json
[ -z "$(osc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
osc create -f examples/sample-app/docker-registry-config.json
[ -n "$(osc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
osc delete -f examples/sample-app/docker-registry-config.json
osc delete imageStreams test
[ -z "$(osc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
osc create -f examples/image-streams/image-streams.json
[ -n "$(osc get imageStreams ruby-20-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams nodejs-010-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams wildfly-8-centos -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams mysql-55-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams postgresql-92-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams mongodb-24-centos7 -t "{{.status.dockerImageRepository}}")" ]
osc delete imageStreams ruby-20-centos7
osc delete imageStreams nodejs-010-centos7
osc delete imageStreams wildfly-8-centos
osc delete imageStreams mysql-55-centos7
osc delete imageStreams postgresql-92-centos7
osc delete imageStreams mongodb-24-centos7
[ -z "$(osc get imageStreams ruby-20-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams nodejs-010-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams wildfly-8-centos -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams mysql-55-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams postgresql-92-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams mongodb-24-centos7 -t "{{.status.dockerImageRepository}}")" ]
echo "imageStreams: ok"

osc create -f test/integration/fixtures/test-image-stream.json
osc create -f test/integration/fixtures/test-image-stream-mapping.json
osc get images
osc get imageStreams
osc get imageStreamTag test:sometag
osc get imageStreamImage test@sha256:4986bf8c15363d1c5d15512d5266f8777bfba4974ac56e3270e7760f6f0a8125
osc delete imageStreams test
echo "imageStreamMappings: ok"

osc get imageRepositories
osc create -f test/integration/fixtures/test-image-repository.json
[ -n "$(osc get imageRepositories test -t "{{.status.dockerImageRepository}}")" ]
osc delete imageRepositories test
osc create -f examples/image-repositories/image-repositories.json
[ -n "$(osc get imageRepositories ruby-20-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories nodejs-010-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories wildfly-8-centos -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories mysql-55-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories postgresql-92-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories mongodb-24-centos7 -t "{{.status.dockerImageRepository}}")" ]
osc delete imageRepositories ruby-20-centos7
osc delete imageRepositories nodejs-010-centos7
osc delete imageRepositories mysql-55-centos7
osc delete imageRepositories postgresql-92-centos7
osc delete imageRepositories mongodb-24-centos7
[ -z "$(osc get imageRepositories ruby-20-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageRepositories nodejs-010-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageRepositories mysql-55-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageRepositories postgresql-92-centos7 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageRepositories mongodb-24-centos7 -t "{{.status.dockerImageRepository}}")" ]
# don't delete wildfly-8-centos
echo "imageRepositories: ok"

osc create -f test/integration/fixtures/test-image-repository.json
osc create -f test/integration/fixtures/test-image-repository-mapping.json
osc get images
osc get imageRepositories
osc get imageRepositoryTag test:sometag
osc delete imageRepositories test
echo "imageRepositoryMappings: ok"

[ "$(osc new-app php mysql -o yaml | grep 3306)" ]
osc new-app php mysql
echo "new-app: ok"

osc get routes
osc create -f test/integration/fixtures/test-route.json
osc delete routes testroute
echo "routes: ok"

osc get deploymentConfigs
osc get dc
osc create -f test/integration/fixtures/test-deployment-config.json
osc describe deploymentConfigs test-deployment-config
osc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

osc process -f test/templates/fixtures/guestbook.json --parameters --value="ADMIN_USERNAME=admin"
osc process -f test/templates/fixtures/guestbook.json | osc create -f -
osc status
[ "$(osc status | grep frontend-service)" ]
echo "template+config: ok"

openshift kube resize --replicas=2 rc guestbook
osc get pods
echo "resize: ok"

osc process -f examples/sample-app/application-template-dockerbuild.json | osc create -f -
osc get buildConfigs
osc get bc
osc get builds
[[ $(osc describe buildConfigs ruby-sample-build | grep --text "Webhook Github") =~ "${API_SCHEME}://${API_HOST}:${API_PORT}/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/github" ]]
[[ $(osc describe buildConfigs ruby-sample-build | grep --text "Webhook Generic") =~ "${API_SCHEME}://${API_HOST}:${API_PORT}/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic" ]]
echo "buildConfig: ok"

osc create -f test/integration/fixtures/test-buildcli.json
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
started=$(osc start-build ruby-sample-build-invalidtag)
echo "start-build: ok"
osc describe build ${started} | grep openshift/ruby-20-centos7$

osc cancel-build "${started}" --dump-logs --restart
# a build for which there is an upstream tag in the corresponding imagerepo, so
# the build should use that specific tag of the image instead of the image field
# as defined in the buildconfig
started=$(osc start-build ruby-sample-build-validtag)
osc describe imagestream ruby-20-centos7-buildcli
osc describe build ${started}
osc describe build ${started} | grep openshift/ruby-20-centos7:success$
osc cancel-build "${started}" --dump-logs --restart
echo "cancel-build: ok"

openshift admin policy add-role-to-group cluster-admin system:unauthenticated
openshift admin policy remove-role-from-group cluster-admin system:unauthenticated
openshift admin policy remove-role-from-group-from-project system:unauthenticated
openshift admin policy add-role-to-user cluster-admin system:no-user
openshift admin policy remove-user cluster-admin system:no-user
openshift admin policy remove-user-from-project system:no-user
echo "ex policy: ok"

# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
osadm new-project ui-test-project --admin="createuser"
osadm policy add-role-to-user admin adduser -n ui-test-project
# Make sure project can be listed by osc (after auth cache syncs)
sleep 2 && [ "$(osc get projects | grep 'ui-test-project')" ]
# Make sure users got added
[ "$(osc describe policybinding master -n ui-test-project | grep createuser)" ]
[ "$(osc describe policybinding master -n ui-test-project | grep adduser)" ]
echo "ui-project-commands: ok"

# Test deleting and recreating a project
osadm new-project recreated-project --admin="createuser1"
osc delete project recreated-project
osc delete project recreated-project
osadm new-project recreated-project --admin="createuser2"
osc describe policybinding master -n recreated-project | grep createuser2
echo "ex new-project: ok"

# Test running a router
[ ! "$(osadm router | grep 'does not exist')" ]
[ "$(osadm router -o yaml --credentials="${OPENSHIFTCONFIG}" | grep 'openshift/origin-haproxy-')" ]
osadm router --create --credentials="${OPENSHIFTCONFIG}"
[ "$(osadm router | grep 'service exists')" ]
echo "ex router: ok"

# Test running a registry
[ ! "$(osadm registry | grep 'does not exist')"]
[ "$(osadm registry -o yaml --credentials="${OPENSHIFTCONFIG}" | grep 'openshift/origin-docker-registry')" ]
osadm registry --create --credentials="${OPENSHIFTCONFIG}"
[ "$(osadm registry | grep 'service exists')" ]
echo "ex registry: ok"

# verify the image repository had its tags populated
[ -n "$(osc get imageStreams wildfly-8-centos -t "{{.status.tags.latest}}")" ]
[ -n "$(osc get imageStreams wildfly-8-centos -t "{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}")" ]

# Test building a dependency tree
[ "$(openshift ex build-chain --all -o dot | grep 'graph')" ]
echo "ex build-chain: ok"

osc get minions,pods

osadm new-project example --admin="createuser"
osc project example
osc create -f test/fixtures/app-scenarios
osc status
echo "complex-scenarios: ok"