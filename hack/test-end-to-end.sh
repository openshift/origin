#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

if [[ -z "$(which iptables)" ]]; then
	echo "IPTables not found - the end-to-end test requires a system with iptables for Kubernetes services."
	exit 1
fi
iptables --list > /dev/null 2>&1
if [ $? -ne 0 ]; then
	sudo iptables --list > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		echo "You do not have iptables or sudo privileges.	Kubernetes services will not work without iptables access.	See https://github.com/GoogleCloudPlatform/kubernetes/issues/1859.	Try 'sudo hack/test-end-to-end.sh'."
		exit 1
	fi
fi

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

echo "[INFO] Starting end-to-end test"

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
	USE_IMAGES='openshift/origin-${component}:latest'
	if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
		COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
		USE_IMAGES="openshift/origin-\${component}:${COMMIT}"
	fi
fi

ROUTER_TESTS_ENABLED="${ROUTER_TESTS_ENABLED:-true}"
TEST_ASSETS="${TEST_ASSETS:-false}"

if [[ -z "${BASETMPDIR-}" ]]; then
	TMPDIR="${TMPDIR:-"/tmp"}"
	BASETMPDIR="${TMPDIR}/openshift-e2e"
	sudo rm -rf "${BASETMPDIR}"
	mkdir -p "${BASETMPDIR}"
fi
ETCD_DATA_DIR="${BASETMPDIR}/etcd"
VOLUME_DIR="${BASETMPDIR}/volumes"
FAKE_HOME_DIR="${BASETMPDIR}/openshift.local.home"
LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
mkdir -p $LOG_DIR
mkdir -p $ARTIFACT_DIR

DEFAULT_SERVER_IP=`ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}'`
API_HOST="${API_HOST:-${DEFAULT_SERVER_IP}}"
API_PORT="${API_PORT:-8443}"
API_SCHEME="${API_SCHEME:-https}"
MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
KUBELET_SCHEME="${KUBELET_SCHEME:-https}"
KUBELET_HOST="${KUBELET_HOST:-127.0.0.1}"
KUBELET_PORT="${KUBELET_PORT:-10250}"

SERVER_CONFIG_DIR="${BASETMPDIR}/openshift.local.config"
MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# used as a resolve IP to test routing
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"
GO_OUT="${OS_ROOT}/_output/local/go/bin"

# set path so OpenShift is available
export PATH="${GO_OUT}:${PATH}"


function cleanup()
{
	out=$?
	echo
	if [ $out -ne 0 ]; then
		echo "[FAIL] !!!!! Test Failed !!!!"
	else
		echo "[INFO] Test Succeeded"
	fi
	echo

	set +e
	echo "[INFO] Dumping container logs to ${LOG_DIR}"
	for container in $(docker ps -aq); do
		docker logs "$container" >&"${LOG_DIR}/container-$container.log"
	done

	echo "[INFO] Dumping build log to ${LOG_DIR}"

	osc get -n test builds --output-version=v1beta1 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n test >"${LOG_DIR}/stibuild.log"
	osc get -n docker builds --output-version=v1beta1 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n docker >"${LOG_DIR}/dockerbuild.log"
	osc get -n custom builds --output-version=v1beta1 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n custom >"${LOG_DIR}/custombuild.log"

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	curl ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:4001/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Tearing down test"
		pids="$(jobs -pr)"
		echo "[INFO] Children: ${pids}"
		sudo kill ${pids}
		sudo ps f
		set +u
		echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
		if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
			echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
		fi
		set -u
	fi
	set -e

	# clean up zero byte log files
	# Clean up large log files so they don't end up on jenkins
	find ${ARTIFACT_DIR} -name *.log -size +20M -exec echo Deleting {} because it is too big. \; -exec rm -f {} \;
	find ${LOG_DIR} -name *.log -size +20M -exec echo Deleting {} because it is too big. \; -exec rm -f {} \;
	find ${LOG_DIR} -name *.log -size 0 -exec echo Deleting {} because it is empty. \; -exec rm -f {} \;

	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

function wait_for_app() {
	echo "[INFO] Waiting for app in namespace $1"
	echo "[INFO] Waiting for database pod to start"
	wait_for_command "osc get -n $1 pods -l name=database | grep -i Running" $((60*TIME_SEC))

	echo "[INFO] Waiting for database service to start"
	wait_for_command "osc get -n $1 services | grep database" $((20*TIME_SEC))
	DB_IP=$(osc get -n $1 --output-version=v1beta1 --template="{{ .portalIP }}" service database)

	echo "[INFO] Waiting for frontend pod to start"
	wait_for_command "osc get -n $1 pods | grep frontend | grep -i Running" $((120*TIME_SEC))

	echo "[INFO] Waiting for frontend service to start"
	wait_for_command "osc get -n $1 services | grep frontend" $((20*TIME_SEC))
	FRONTEND_IP=$(osc get -n $1 --output-version=v1beta1 --template="{{ .portalIP }}" service frontend)

	echo "[INFO] Waiting for database to start..."
	wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " $((3*TIME_MIN))

	echo "[INFO] Waiting for app to start..."
	wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))

	echo "[INFO] Testing app"
	wait_for_command '[[ "$(curl -s -X POST http://${FRONTEND_IP}:5432/keys/foo -d value=1337)" = "Key created" ]]'
	wait_for_command '[[ "$(curl -s http://${FRONTEND_IP}:5432/keys/foo)" = "1337" ]]'
}

# Wait for builds to complete
# $1 namespace
function wait_for_build() {
	echo "[INFO] Waiting for $1 namespace build to complete"
	wait_for_command "osc get -n $1 builds | grep -i complete" $((10*TIME_MIN)) "osc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`osc get -n $1 builds --output-version=v1beta1 -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} finished"
  # TODO: fix
  set +e
	osc build-logs -n $1 $BUILD_ID > $LOG_DIR/$1build.log
  set -e
}

# Setup
stop_openshift_server
echo "[INFO] `openshift version`"
echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
echo "[INFO] Using images:              ${USE_IMAGES}"

# Start All-in-one server and wait for health
echo "[INFO] Create certificates for the OpenShift server"
# find the same IP that openshift start will bind to.  This allows access from pods that have to talk back to master
ALL_IP_ADDRESSES=`ifconfig | grep "inet " | sed 's/adr://' | awk '{print $2}'`
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost"
while read -r IP_ADDRESS
do
	SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
done <<< "${ALL_IP_ADDRESSES}"

openshift admin create-master-certs \
	--overwrite=false \
	--cert-dir="${MASTER_CONFIG_DIR}" \
	--hostnames="${SERVER_HOSTNAME_LIST}" \
	--master="${MASTER_ADDR}" \
	--public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

openshift admin create-node-config \
	--listen="${KUBELET_SCHEME}://0.0.0.0:${KUBELET_PORT}" \
	--node-dir="${NODE_CONFIG_DIR}" \
	--node="${KUBELET_HOST}" \
	--hostnames="${KUBELET_HOST}" \
	--master="${MASTER_ADDR}" \
	--node-client-certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
	--certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
	--signer-cert="${MASTER_CONFIG_DIR}/ca.crt" \
	--signer-key="${MASTER_CONFIG_DIR}/ca.key" \
	--signer-serial="${MASTER_CONFIG_DIR}/ca.serial.txt"

osadm create-bootstrap-policy-file --filename="${MASTER_CONFIG_DIR}/policy.json"

# create openshift config
openshift start \
	--write-config=${SERVER_CONFIG_DIR} \
	--create-certs=false \
    --listen="${API_SCHEME}://0.0.0.0:${API_PORT}" \
    --master="${MASTER_ADDR}" \
    --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}" \
    --hostname="${KUBELET_HOST}" \
    --volume-dir="${VOLUME_DIR}" \
    --etcd-dir="${ETCD_DATA_DIR}" \
    --images="${USE_IMAGES}"


echo "[INFO] Starting OpenShift server"
sudo env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start \
	--master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
	--node-config=${NODE_CONFIG_DIR}/node-config.yaml \
    --loglevel=4 \
    &> "${LOG_DIR}/openshift.log" &
OS_PID=$!

export HOME="${FAKE_HOME_DIR}"
# This directory must exist so Docker can store credentials in $HOME/.dockercfg
mkdir -p ${FAKE_HOME_DIR}

if [[ "${API_SCHEME}" == "https" ]]; then
	export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
	export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
	export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

	# Make osc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	export OPENSHIFTCONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
	sudo chmod -R a+rwX "${OPENSHIFTCONFIG}"
	echo "[INFO] To debug: export OPENSHIFTCONFIG=$OPENSHIFTCONFIG"
fi

wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta3/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

# add e2e-user as a viewer for the default namespace so we can see infrastructure pieces appear
openshift admin policy add-role-to-user view e2e-user --namespace=default

# create test project so that this shows up in the console
openshift admin new-project test --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"
openshift admin new-project docker --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"
openshift admin new-project custom --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"
openshift admin new-project cache --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"

echo "The console should be available at ${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}/console."
echo "Log in as 'e2e-user' to see the 'test' project."

# install the router
echo "[INFO] Installing the router"
openshift admin router --create --credentials="${MASTER_CONFIG_DIR}/openshift-router.kubeconfig" --images="${USE_IMAGES}"

# install the registry. The --mount-host option is provided to reuse local storage.
echo "[INFO] Installing the registry"
openshift admin registry --create --credentials="${MASTER_CONFIG_DIR}/openshift-registry.kubeconfig" --mount-host="/tmp/openshift.local.registry" --images="${USE_IMAGES}"

echo "[INFO] Pre-pulling and pushing ruby-20-centos7"
docker pull openshift/ruby-20-centos7:latest
echo "[INFO] Pulled ruby-20-centos7"

echo "[INFO] Waiting for Docker registry pod to start"
# TODO: simplify when #4702 is fixed upstream
wait_for_command '[[ "$(osc get endpoints docker-registry --output-version=v1beta1 -t "{{ if .endpoints }}{{ len .endpoints }}{{ else }}0{{ end }}" || echo "0")" != "0" ]]' $((5*TIME_MIN))

# services can end up on any IP.	Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY=$(osc get --output-version=v1beta3 --template="{{ .spec.portalIP }}:{{ with index .spec.ports 0 }}{{ .port }}{{ end }}" service docker-registry)

echo "[INFO] Verifying the docker-registry is up at ${DOCKER_REGISTRY}"
wait_for_url_timed "http://${DOCKER_REGISTRY}/healthz" "[INFO] Docker registry says: " $((2*TIME_MIN))

[ "$(dig @${API_HOST} "docker-registry.default.local." A)" ]

# Client setup (log in as e2e-user and set 'test' as the default project)
# This is required to be able to push to the registry!
echo "[INFO] Logging in as a regular user (e2e-user:pass) with project 'test'..."
osc login -u e2e-user -p pass
osc project cache
token=$(osc config view --flatten -o template -t '{{with index .users 0}}{{.user.token}}{{end}}')
[[ -n ${token} ]]

echo "[INFO] Docker login as e2e-user to ${DOCKER_REGISTRY}"
docker login -u e2e-user -p ${token} -e e2e-user@openshift.com ${DOCKER_REGISTRY}
echo "[INFO] Docker login successful"

echo "[INFO] Tagging and pushing ruby-20-centos7 to ${DOCKER_REGISTRY}/cache/ruby-20-centos7:latest"
docker tag -f openshift/ruby-20-centos7:latest ${DOCKER_REGISTRY}/cache/ruby-20-centos7:latest
docker push ${DOCKER_REGISTRY}/cache/ruby-20-centos7:latest
echo "[INFO] Pushed ruby-20-centos7"

echo "[INFO] Back to 'master' context with 'admin' user..."
osc project default

# The build requires a dockercfg secret in the builder service account in order
# to be able to push to the registry.  Make sure it exists first.
echo "[INFO] Waiting for dockercfg secrets to be generated in project 'test' before building"
wait_for_command "osc get -n test serviceaccount/builder -o yaml | grep dockercfg > /dev/null" $((60*TIME_SEC))

# Process template and create
echo "[INFO] Submitting application template json for processing..."
osc process -n test -f examples/sample-app/application-template-stibuild.json > "${STI_CONFIG_FILE}"
osc process -n docker -f examples/sample-app/application-template-dockerbuild.json > "${DOCKER_CONFIG_FILE}"
osc process -n custom -f examples/sample-app/application-template-custombuild.json > "${CUSTOM_CONFIG_FILE}"

echo "[INFO] Back to 'test' context with 'e2e-user' user"
osc project test

echo "[INFO] Applying STI application config"
osc create -f "${STI_CONFIG_FILE}"


# this needs to be done before waiting for the build because right now only cluster-admins can see build logs, because that uses proxy
echo "[INFO] Back to 'master' context with 'admin' user..."
osc project default

# Trigger build
echo "[INFO] Starting build from ${STI_CONFIG_FILE} and streaming its logs..."
#osc start-build -n test ruby-sample-build --follow
wait_for_build "test"
wait_for_app "test"

#echo "[INFO] Applying Docker application config"
#osc create -n docker -f "${DOCKER_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new docker build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=docker && sleep 3
#wait_for_build "docker"
#wait_for_app "docker"

#echo "[INFO] Applying Custom application config"
#osc create -n custom -f "${CUSTOM_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new custom build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=custom && sleep 3
#wait_for_build "custom"
#wait_for_app "custom"

# ensure the router is started
# TODO: simplify when #4702 is fixed upstream
wait_for_command '[[ "$(osc get endpoints router --output-version=v1beta1 -t "{{ if .endpoints }}{{ len .endpoints }}{{ else }}0{{ end }}" || echo "0")" != "0" ]]' $((5*TIME_MIN))

echo "[INFO] Validating routed app response..."
validate_response "-s -k --resolve www.example.com:443:${CONTAINER_ACCESSIBLE_API_HOST} https://www.example.com" "Hello from OpenShift" 0.2 50

# Remote command execution
echo "[INFO] Validating exec"
registry_pod=$(osc get pod | grep deployment=docker-registry | grep docker-registry | awk '{print $1}')
osc exec -p ${registry_pod} whoami | grep root

# Port forwarding
echo "[INFO] Validating port-forward"
osc port-forward -p ${registry_pod} 5001:5000  &> "${LOG_DIR}/port-forward.log" &
wait_for_url_timed "http://localhost:5001/healthz" "[INFO] Docker registry says: " $((10*TIME_SEC))

# UI e2e tests can be found in assets/test/e2e
if [[ "$TEST_ASSETS" == "true" ]]; then
	echo "[INFO] Running UI e2e tests..."
	pushd ${OS_ROOT}/assets > /dev/null
		grunt test-e2e
	popd > /dev/null
fi
