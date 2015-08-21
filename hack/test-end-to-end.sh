#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

test_privileges

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


TEST_TYPE="openshift-e2e"
TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/${TEST_TYPE}"

if [[ -d "${BASETMPDIR}" ]]; then
	remove_tmp_dir $TEST_TYPE && mkdir -p "${BASETMPDIR}"
fi

LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
DEFAULT_SERVER_IP=`ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}'`
API_HOST="${API_HOST:-${DEFAULT_SERVER_IP}}"
setup_env_vars
mkdir -p $LOG_DIR $ARTIFACT_DIR

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# used as a resolve IP to test routing
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"


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
	dump_container_logs

	echo "[INFO] Dumping build log to ${LOG_DIR}"

	oc get -n test builds --output-version=v1beta3 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l oc build-logs -n test >"${LOG_DIR}/stibuild.log"
	oc get -n docker builds --output-version=v1beta3 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l oc build-logs -n docker >"${LOG_DIR}/dockerbuild.log"
	oc get -n custom builds --output-version=v1beta3 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l oc build-logs -n custom >"${LOG_DIR}/custombuild.log"

	oc export all --all-namespaces --raw -o json > ${LOG_DIR}/export_all.json

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	curl ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:4001/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Switch back to 'default' project with 'admin' user for cleanup"
		oc project ${CLUSTER_ADMIN_CONTEXT}

		echo "[INFO] Deleting test constructs"
		oc delete -n test all --all
		oc delete -n docker all --all
		oc delete -n custom all --all
		oc delete -n cache all --all
		oc delete -n default all --all

		echo "[INFO] Tearing down test"
		kill_all_processes

		set +u
		echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
		if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
			echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
		fi
		set -u
	fi
	set -e

	delete_large_and_empty_logs

	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

function wait_for_app() {
	echo "[INFO] Waiting for app in namespace $1"
	echo "[INFO] Waiting for database pod to start"
	wait_for_command "oc get -n $1 pods -l name=database | grep -i Running" $((60*TIME_SEC))

	echo "[INFO] Waiting for database service to start"
	wait_for_command "oc get -n $1 services | grep database" $((20*TIME_SEC))
	DB_IP=$(oc get -n $1 --output-version=v1beta3 --template="{{ .spec.portalIP }}" service database)

	echo "[INFO] Waiting for frontend pod to start"
	wait_for_command "oc get -n $1 pods | grep frontend | grep -i Running" $((120*TIME_SEC))

	echo "[INFO] Waiting for frontend service to start"
	wait_for_command "oc get -n $1 services | grep frontend" $((20*TIME_SEC))
	FRONTEND_IP=$(oc get -n $1 --output-version=v1beta3 --template="{{ .spec.portalIP }}" service frontend)

	echo "[INFO] Waiting for database to start..."
	wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " $((3*TIME_MIN))

	echo "[INFO] Waiting for app to start..."
	wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))

	echo "[INFO] Testing app"
	wait_for_command '[[ "$(curl -s -X POST http://${FRONTEND_IP}:5432/keys/foo -d value=1337)" = "Key created" ]]'
	wait_for_command '[[ "$(curl -s http://${FRONTEND_IP}:5432/keys/foo)" = "1337" ]]'
}

# Wait for builds to start
# $1 namespace
function wait_for_build_start() {
        echo "[INFO] Waiting for $1 namespace build to start"
        wait_for_command "oc get -n $1 builds | grep -i running" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
        BUILD_ID=`oc get -n $1 builds  --output-version=v1 -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
        echo "[INFO] Build ${BUILD_ID} started"
}

# Wait for builds to complete
# $1 namespace
function wait_for_build() {
	echo "[INFO] Waiting for $1 namespace build to complete"
	wait_for_command "oc get -n $1 builds | grep -i complete" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`oc get -n $1 builds --output-version=v1beta3 -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} finished"
  # TODO: fix
  set +e
	oc build-logs -n $1 $BUILD_ID > $LOG_DIR/$1build.log
  set -e
}

# Setup
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

configure_os_server

export HOME="${FAKE_HOME_DIR}"
# This directory must exist so Docker can store credentials in $HOME/.dockercfg
mkdir -p ${FAKE_HOME_DIR}

export KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
CLUSTER_ADMIN_CONTEXT=$(oc config view --flatten -o template -t '{{index . "current-context"}}')

if [[ "${API_SCHEME}" == "https" ]]; then
	export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
	export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
	export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

	# Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	sudo chmod -R a+rwX "${KUBECONFIG}"
	echo "[INFO] To debug: export KUBECONFIG=$KUBECONFIG"
fi

start_os_server

# add e2e-user as a viewer for the default namespace so we can see infrastructure pieces appear
openshift admin policy add-role-to-user view e2e-user --namespace=default

# pre-load some image streams and templates
oc create -f examples/image-streams/image-streams-centos7.json --namespace=openshift
oc create -f examples/sample-app/application-template-stibuild.json --namespace=openshift

# create test project so that this shows up in the console
openshift admin new-project test --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"
openshift admin new-project docker --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"
openshift admin new-project custom --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"
openshift admin new-project cache --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"

echo "The console should be available at ${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}/console."
echo "Log in as 'e2e-user' to see the 'test' project."

# install the router
echo "[INFO] Installing the router"
echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f -
oc get scc privileged -o json | sed '/\"users\"/a \"system:serviceaccount:default:router\",' | oc replace scc privileged -f -
openshift admin router --create --credentials="${MASTER_CONFIG_DIR}/openshift-router.kubeconfig" --images="${USE_IMAGES}" --service-account=router

# install the registry. The --mount-host option is provided to reuse local storage.
echo "[INFO] Installing the registry"
openshift admin registry --create --credentials="${MASTER_CONFIG_DIR}/openshift-registry.kubeconfig" --images="${USE_IMAGES}"

echo "[INFO] Pre-pulling and pushing ruby-20-centos7"
docker pull openshift/ruby-20-centos7:latest
echo "[INFO] Pulled ruby-20-centos7"

echo "[INFO] Waiting for Docker registry pod to start"
# TODO: simplify when #4702 is fixed upstream
wait_for_command '[[ "$(oc get endpoints docker-registry --output-version=v1beta3 -t "{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}" || echo "0")" != "0" ]]' $((5*TIME_MIN))

# services can end up on any IP.	Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY=$(oc get --output-version=v1beta3 --template="{{ .spec.portalIP }}:{{ with index .spec.ports 0 }}{{ .port }}{{ end }}" service docker-registry)

registry="$(dig @${API_HOST} "docker-registry.default.svc.cluster.local." +short A | head -n 1)"
[[ -n "${registry}" && "${registry}:5000" == "${DOCKER_REGISTRY}" ]]

echo "[INFO] Verifying the docker-registry is up at ${DOCKER_REGISTRY}"
wait_for_url_timed "http://${DOCKER_REGISTRY}/healthz" "[INFO] Docker registry says: " $((2*TIME_MIN))

[ "$(dig @${API_HOST} "docker-registry.default.local." A)" ]

# Client setup (log in as e2e-user and set 'test' as the default project)
# This is required to be able to push to the registry!
echo "[INFO] Logging in as a regular user (e2e-user:pass) with project 'test'..."
oc login -u e2e-user -p pass
[ "$(oc whoami | grep 'e2e-user')" ]
 
# make sure viewers can see oc status
oc status -n default

oc project cache
token=$(oc config view --flatten -o template -t '{{with index .users 0}}{{.user.token}}{{end}}')
[[ -n ${token} ]]

echo "[INFO] Docker login as e2e-user to ${DOCKER_REGISTRY}"
docker login -u e2e-user -p ${token} -e e2e-user@openshift.com ${DOCKER_REGISTRY}
echo "[INFO] Docker login successful"

echo "[INFO] Tagging and pushing ruby-20-centos7 to ${DOCKER_REGISTRY}/cache/ruby-20-centos7:latest"
docker tag -f openshift/ruby-20-centos7:latest ${DOCKER_REGISTRY}/cache/ruby-20-centos7:latest
docker push ${DOCKER_REGISTRY}/cache/ruby-20-centos7:latest
echo "[INFO] Pushed ruby-20-centos7"

echo "[INFO] Back to 'default' project with 'admin' user..."
oc project ${CLUSTER_ADMIN_CONTEXT}
[ "$(oc whoami | grep 'system:admin')" ]

# The build requires a dockercfg secret in the builder service account in order
# to be able to push to the registry.  Make sure it exists first.
echo "[INFO] Waiting for dockercfg secrets to be generated in project 'test' before building"
wait_for_command "oc get -n test serviceaccount/builder -o yaml | grep dockercfg > /dev/null" $((60*TIME_SEC))

# Process template and create
echo "[INFO] Submitting application template json for processing..."
oc process -n test -f examples/sample-app/application-template-stibuild.json > "${STI_CONFIG_FILE}"
oc process -n docker -f examples/sample-app/application-template-dockerbuild.json > "${DOCKER_CONFIG_FILE}"
oc process -n custom -f examples/sample-app/application-template-custombuild.json > "${CUSTOM_CONFIG_FILE}"

echo "[INFO] Back to 'test' context with 'e2e-user' user"
oc login -u e2e-user
oc project test
oc whoami

echo "[INFO] Applying STI application config"
oc create -f "${STI_CONFIG_FILE}"

# Wait for build which should have triggered automatically
echo "[INFO] Starting build from ${STI_CONFIG_FILE} and streaming its logs..."
#oc start-build -n test ruby-sample-build --follow
wait_for_build_start "test"
# Ensure that the build pod doesn't allow exec
[ "$(oc rsh ${BUILD_ID}-build 2>&1 | grep 'forbidden')" ]
wait_for_build "test"
wait_for_app "test"

# Remote command execution
echo "[INFO] Validating exec"
frontend_pod=$(oc get pod -l deploymentconfig=frontend -t '{{(index .items 0).metadata.name}}')
# when running as a restricted pod the registry will run with a pre-allocated
# user in the neighborhood of 1000000+.  Look for a substring of the pre-allocated uid range
oc exec -p ${frontend_pod} id | grep 10

# Port forwarding
echo "[INFO] Validating port-forward"
oc port-forward -p ${frontend_pod} 10080:8080  &> "${LOG_DIR}/port-forward.log" &
wait_for_url_timed "http://localhost:10080" "[INFO] Frontend says: " $((10*TIME_SEC))



#echo "[INFO] Applying Docker application config"
#oc create -n docker -f "${DOCKER_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new docker build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta3/namespaces/docker/buildconfigs/ruby-sample-build/webhooks/secret101/generic && sleep 3
#wait_for_build "docker"
#wait_for_app "docker"

#echo "[INFO] Applying Custom application config"
#oc create -n custom -f "${CUSTOM_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new custom build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta3/namespaces/custom/buildconfigs/ruby-sample-build/webhooks/secret101/generic && sleep 3
#wait_for_build "custom"
#wait_for_app "custom"

echo "[INFO] Back to 'default' project with 'admin' user..."
oc project ${CLUSTER_ADMIN_CONTEXT}

# ensure the router is started
# TODO: simplify when #4702 is fixed upstream
wait_for_command '[[ "$(oc get endpoints router --output-version=v1beta3 -t "{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}" || echo "0")" != "0" ]]' $((5*TIME_MIN))

echo "[INFO] Validating routed app response..."
validate_response "-s -k --resolve www.example.com:443:${CONTAINER_ACCESSIBLE_API_HOST} https://www.example.com" "Hello from OpenShift" 0.2 50


# Pod node selection
echo "[INFO] Validating pod.spec.nodeSelector rejections"
# Create a project that enforces an impossible to satisfy nodeSelector, and two pods, one of which has an explicit node name
openshift admin new-project node-selector --description="This is an example project to test node selection prevents deployment" --admin="e2e-user" --node-selector="impossible-label=true"
oc process -n node-selector -v NODE_NAME="${KUBELET_HOST}" -f test/node-selector/pods.json | oc create -n node-selector -f -
# The pod without a node name should fail to schedule
wait_for_command "oc get events -n node-selector | grep pod-without-node-name | grep failedScheduling"        $((20*TIME_SEC))
# The pod with a node name should be rejected by the kubelet
wait_for_command "oc get events -n node-selector | grep pod-with-node-name    | grep NodeSelectorMismatching" $((20*TIME_SEC))


# Image pruning
echo "[INFO] Validating image pruning"
docker pull busybox
docker pull gcr.io/google_containers/pause
docker pull openshift/hello-openshift

# tag and push 1st image - layers unique to this image will be pruned
docker tag -f busybox ${DOCKER_REGISTRY}/cache/prune
docker push ${DOCKER_REGISTRY}/cache/prune

# tag and push 2nd image - layers unique to this image will be pruned
docker tag -f openshift/hello-openshift ${DOCKER_REGISTRY}/cache/prune
docker push ${DOCKER_REGISTRY}/cache/prune

# tag and push 3rd image - it won't be pruned
docker tag -f gcr.io/google_containers/pause ${DOCKER_REGISTRY}/cache/prune
docker push ${DOCKER_REGISTRY}/cache/prune

# record the storage before pruning
registry_pod=$(oc get pod -l deploymentconfig=docker-registry -t '{{(index .items 0).metadata.name}}')
oc exec -p ${registry_pod} du /registry > ${LOG_DIR}/prune-images.before.txt

# set up pruner user
oadm policy add-cluster-role-to-user system:image-pruner e2e-pruner
oc login -u e2e-pruner -p pass

# run image pruning
oadm prune images --keep-younger-than=0 --keep-tag-revisions=1 --confirm &> ${LOG_DIR}/prune-images.log
! grep error ${LOG_DIR}/prune-images.log

oc project ${CLUSTER_ADMIN_CONTEXT}
# record the storage after pruning
oc exec -p ${registry_pod} du /registry > ${LOG_DIR}/prune-images.after.txt

# make sure there were changes to the registry's storage
[ -n "$(diff ${LOG_DIR}/prune-images.before.txt ${LOG_DIR}/prune-images.after.txt)" ]

# UI e2e tests can be found in assets/test/e2e
if [[ "$TEST_ASSETS" == "true" ]]; then

	if [[ "$TEST_ASSETS_HEADLESS" == "true" ]]; then
		echo "[INFO] Starting virtual framebuffer for headless tests..."
		export DISPLAY=:10
		Xvfb :10 -screen 0 1024x768x24 -ac &
	fi

	echo "[INFO] Running UI e2e tests..."
	pushd ${OS_ROOT}/assets > /dev/null
		grunt test-e2e
	popd > /dev/null
fi
