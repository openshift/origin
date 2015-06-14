#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

echo "[INFO] Starting containerized end-to-end test"

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
  tag="latest"
  if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
    COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
    tag="${COMMIT}"
  fi
  USE_IMAGES="openshift/origin-\${component}:${tag}"
fi

unset KUBECONFIG

if [[ -z "${BASETMPDIR-}" ]]; then
  TMPDIR="${TMPDIR:-"/tmp"}"
  BASETMPDIR="${TMPDIR}/openshift-e2e/containerized"
  sudo rm -rf "${BASETMPDIR}"
  mkdir -p "${BASETMPDIR}"
fi
VOLUME_DIR="${BASETMPDIR}/volumes"
FAKE_HOME_DIR="${BASETMPDIR}/openshift.local.home"
LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
mkdir -p $LOG_DIR
mkdir -p $ARTIFACT_DIR

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
  docker logs origin >"${LOG_DIR}/openshift.log" 2>&1

  if [[ -z "${SKIP_TEARDOWN-}" ]]; then
    echo "[INFO] Tearing down test"
    docker stop origin
    docker rm origin

    echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
    if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
      echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
    fi
    set -u
  fi
  set -e

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

out=$(
  set +e
  docker stop origin 2>&1
  docker rm origin 2>&1
  set -e
)

# Setup
echo "[INFO] `openshift version`"
echo "[INFO] Using images:              ${USE_IMAGES}"

echo "[INFO] Starting OpenShift containerized server"
sudo docker run -d --name="origin" \
  --privileged --net=host \
  -v /:/rootfs:ro -v /var/run:/var/run:rw -v /sys:/sys:ro -v /var/lib/docker:/var/lib/docker:rw \
  -v "/var/lib/openshift/openshift.local.volumes:/var/lib/openshift/openshift.local.volumes" \
  "openshift/origin:${tag}" start

export HOME="${FAKE_HOME_DIR}"
# This directory must exist so Docker can store credentials in $HOME/.dockercfg
mkdir -p ${FAKE_HOME_DIR}

CURL_EXTRA="-k"

wait_for_url "https://localhost:8443/healthz/ready" "apiserver(ready): " 0.25 160

# install the router
echo "[INFO] Installing the router"
sudo docker exec origin openshift admin router --create --credentials="./openshift.local.config/master/openshift-router.kubeconfig" --images="${USE_IMAGES}"

# install the registry. The --mount-host option is provided to reuse local storage.
echo "[INFO] Installing the registry"
sudo docker exec origin openshift admin registry --create --credentials="./openshift.local.config/master/openshift-registry.kubeconfig" --images="${USE_IMAGES}"

registry="$(dig @localhost "docker-registry.default.svc.cluster.local." +short A | head -n 1)"
[ -n "${registry}" ]
echo "[INFO] Verifying the docker-registry is up at ${registry}"
wait_for_url_timed "http://${registry}:5000/healthz" "[INFO] Docker registry says: " $((2*TIME_MIN))


echo "[INFO] Login"
oc login localhost:8443 -u test -p test --insecure-skip-tls-verify
oc new-project test

echo "[INFO] Applying STI application config"
oc new-app -f examples/sample-app/application-template-stibuild.json

# Wait for build which should have triggered automatically
echo "[INFO] Starting build..."
#oc start-build -n test ruby-sample-build --follow
wait_for_build "test"
wait_for_app "test"

