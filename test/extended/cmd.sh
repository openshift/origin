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
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

source "${OS_ROOT}/hack/lib/util/environment.sh"
os::util::environment::setup_time_vars

cd "${OS_ROOT}"

os::build::setup_env

function cleanup()
{
	out=$?
	docker rmi test/scratchimage
	cleanup_openshift
	echo "[INFO] Exiting"
	return $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

echo "[INFO] Starting server"

os::util::environment::setup_all_server_vars "test-extended/cmd/"
os::util::environment::use_sudo
reset_tmp_dir

os::log::start_system_logger

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
os::cmd::expect_success "oc delete all --all"
IMAGE_NS_TOKEN=$(oc sa get-token builder)
os::cmd::expect_success "docker login -u imagensbuilder -p ${IMAGE_NS_TOKEN} -e fake@example.org ${DOCKER_REGISTRY}"
os::cmd::expect_success "oc import-image busybox:latest --confirm"
os::cmd::expect_success "docker pull busybox"
os::cmd::expect_success "docker tag -f docker.io/busybox:latest ${DOCKER_REGISTRY}/image-ns/busybox:latest"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/image-ns/busybox:latest"
os::cmd::expect_success "docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest"


DOCKER_CONFIG_JSON=${HOME}/.docker/config.json
oc new-project dc-ns
os::cmd::expect_success "oc delete all --all"
os::cmd::expect_success "oc delete secrets --all"
os::cmd::expect_success "oc secrets new image-ns-pull .dockerconfigjson=${DOCKER_CONFIG_JSON}"
os::cmd::expect_success "oc secrets new-dockercfg image-ns-pull-old --docker-email=fake@example.org --docker-username=imagensbuilder --docker-server=${DOCKER_REGISTRY} --docker-password=${IMAGE_NS_TOKEN}"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/pod-with-no-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - "
os::cmd::try_until_text "oc describe pod/no-pull-pod" 'Back-off pulling image'
os::cmd::expect_success "oc delete pods --all"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/pod-with-new-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - "
os::cmd::try_until_text 'oc get pods/new-pull-pod -o jsonpath={.status.containerStatuses[0].imageID}' 'docker'
os::cmd::expect_success "oc delete pods --all"
os::cmd::expect_success "docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/pod-with-old-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - "
os::cmd::try_until_text 'oc get pods/old-pull-pod -o jsonpath={.status.containerStatuses[0].imageID}' 'docker'
os::cmd::expect_success "oc delete pods --all"
os::cmd::expect_success "docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/dc-with-old-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - "
os::cmd::try_until_text 'oc get pods/my-dc-old-1-hook-pre -o jsonpath={.status.containerStatuses[0].imageID}' 'docker'
os::cmd::expect_success "oc delete all --all"
os::cmd::expect_success "docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/dc-with-new-pull-secret.yaml --value=DOCKER_REGISTRY=${DOCKER_REGISTRY} | oc create -f - "
os::cmd::try_until_text 'oc get pods/my-dc-1-hook-pre -o jsonpath={.status.containerStatuses[0].imageID}' 'docker'
os::cmd::expect_success "oc delete all --all"
os::cmd::expect_success "docker rmi -f ${DOCKER_REGISTRY}/image-ns/busybox:latest"
