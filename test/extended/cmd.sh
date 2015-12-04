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

os::build::setup_env

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/authentication"
export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"
export KUBE_REPO_ROOT="${OS_ROOT}/../../../k8s.io/kubernetes"

function join { local IFS="$1"; shift; echo "$*"; }


function cleanup()
{
	docker rmi test/scratchimage
	cleanup_openshift
	echo "[INFO] Exiting"
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
# let everyone be able to see stuff in the default namespace
oadm policy add-role-to-group view system:authenticated -n default

install_registry
wait_for_registry
DOCKER_REGISTRY=`oc get service/docker-registry -n default -o jsonpath='{.spec.clusterIP}:{.spec.ports[0].port}'`


echo "[INFO] Running newapp extended tests"
oc login ${MASTER_ADDR} -u new-app -p password --certificate-authority=${MASTER_CONFIG_DIR}/ca.crt
oc new-project new-app
oc delete all --all

# create a local-only docker image for testing
# image is removed in cleanup()
tmp=$(mktemp -d)
pushd $tmp
cat <<-EOF >> Dockerfile
	FROM scratch
	EXPOSE 80
EOF
docker build -t test/scratchimage .
popd
rm -rf $tmp

# ensure a local-only image gets a docker image(not imagestream) reference created.
[ "$(oc new-app test/scratchimage~https://github.com/openshift/ruby-hello-world.git --strategy=docker -o yaml |& tr '\n' ' ' | grep -E "from:\s+kind:\s+DockerImage\s+name:\s+test/scratchimage:latest\s+")" ]
# error due to partial match
[ "$(oc new-app test/scratchimage2 -o yaml |& tr '\n' ' ' 2>&1 | grep -E "partial match")" ]
# success with exact match	
[ "$(oc new-app test/scratchimage -o yaml)" ]
echo "[INFO] newapp: ok"

echo "[INFO] Running env variable expansion tests"
oc new-project envtest
oc create -f test/extended/fixtures/test-env-pod.json
tryuntil "oc get pods | grep Running"
podname=$(oc get pods --template='{{with index .items 0}}{{.metadata.name}}{{end}}')
oc exec test-pod env | grep podname=test-pod
oc exec test-pod env | grep podname_composed=test-pod_composed
oc exec test-pod env | grep var1=value1
oc exec test-pod env | grep var2=value1
oc exec test-pod ps ax | grep "sleep 120"
echo "[INFO] variable-expansion: ok"

echo "[INFO] Running image pull secrets tests"
oc login ${MASTER_ADDR} -u pull-secrets-user -p password --certificate-authority=${MASTER_CONFIG_DIR}/ca.crt

# create a new project and push a busybox image in there
oc new-project image-ns
oc delete all --all
IMAGE_NS_TOKEN=`oc get sa/builder --template='{{range .secrets}}{{ .name }} {{end}}' | xargs -n 1 oc get secret --template='{{ if .data.token }}{{ .data.token }}{{end}}' | base64 -d -`
docker login -u imagensbuilder -p ${IMAGE_NS_TOKEN} -e fake@example.org ${DOCKER_REGISTRY}
oc tag --source=docker busybox:latest image-ns/busybox:latest
oc import-image busybox
docker pull busybox
docker tag -f docker.io/busybox:latest ${DOCKER_REGISTRY}/image-ns/busybox:latest
docker push ${DOCKER_REGISTRY}/image-ns/busybox:latest
docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest


DOCKER_CONFIG_JSON=${HOME}/.docker/config.json
oc new-project dc-ns
oc delete all --all
oc delete secrets --all
oc secrets new image-ns-pull .dockerconfigjson=${DOCKER_CONFIG_JSON}
oc secrets new-dockercfg image-ns-pull-old --docker-email=fake@example.org --docker-username=imagensbuilder --docker-server=${DOCKER_REGISTRY} --docker-password=${IMAGE_NS_TOKEN}

oc process -f test/extended/fixtures/image-pull-secrets/pod-with-no-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - 
wait_for_command "oc describe pod/no-pull-pod | grep 'Back-off pulling image'" 30*TIME_SEC
oc delete pods --all

# TODO remove sleeps once jsonpath stops panicing.  The code still works without the sleep, it just looks nasty

oc process -f test/extended/fixtures/image-pull-secrets/pod-with-new-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - 
sleep 1
wait_for_command "oc get pods/new-pull-pod -o jsonpath='{.status.containerStatuses[0].imageID}' | grep 'docker'" 30*TIME_SEC
oc delete pods --all
docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest


oc process -f test/extended/fixtures/image-pull-secrets/pod-with-old-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - 
sleep 1
wait_for_command "oc get pods/old-pull-pod -o jsonpath={.status.containerStatuses[0].imageID} | grep 'docker'" 30*TIME_SEC
oc delete pods --all
docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest

oc process -f test/extended/fixtures/image-pull-secrets/dc-with-old-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - 
sleep 4
wait_for_command "oc get pods/my-dc-old-1-prehook -o jsonpath={.status.containerStatuses[0].imageID} | grep 'docker'" 30*TIME_SEC
oc delete all --all
docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest

oc process -f test/extended/fixtures/image-pull-secrets/dc-with-new-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - 
sleep 4
wait_for_command "oc get pods/my-dc-1-prehook -o jsonpath={.status.containerStatuses[0].imageID} | grep 'docker'" 30*TIME_SEC
oc delete all --all
docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest
