#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

os::util::environment::setup_time_vars
trap os::test::junit::reconcile_output EXIT

export VERBOSE=true

function wait_for_app() {
  os::log::info "Waiting for app in namespace $1"
  os::log::info "Waiting for database pod to start"
  os::cmd::try_until_text "oc get -n $1 pods -l name=database" 'Running'
  os::cmd::expect_success "oc logs dc/database -n $1"

  os::log::info "Waiting for database service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'database' "$(( 2 * TIME_MIN ))"
  DB_IP=$(oc get -n "$1" --output-version=v1 --template="{{ .spec.clusterIP }}" service database)

  os::log::info "Waiting for frontend pod to start"
  os::cmd::try_until_text "oc get -n $1 pods -l name=frontend" 'Running' "$(( 2 * TIME_MIN ))"
  os::cmd::expect_success "oc logs dc/frontend -n $1"

  os::log::info "Waiting for frontend service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'frontend' "$(( 2 * TIME_MIN ))"
  FRONTEND_IP=$(oc get -n "$1" --output-version=v1 --template="{{ .spec.clusterIP }}" service frontend)

  os::log::info "Waiting for database to start..."
  os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://${DB_IP}:5434'" "$((3*TIME_MIN))"

  os::log::info "Waiting for app to start..."
  os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://${FRONTEND_IP}:5432'" "$((2*TIME_MIN))"

  os::log::info "Testing app"
  os::cmd::try_until_text "curl -s -X POST http://${FRONTEND_IP}:5432/keys/foo -d value=1337" "Key created"
  os::cmd::try_until_text "curl -s http://${FRONTEND_IP}:5432/keys/foo" "1337"
}

function remove_docker_images() {
    local name="$1"
    local tag="${2:-\S\+}"
    local imageids=$(docker images | sed -n "s,^.*$name\s\+$tag\s\+\(\S\+\).*,\1,p" | sort -u | tr '\n' ' ')
    os::cmd::expect_success_and_text "echo '${imageids}' | wc -w" '^[1-9][0-9]*$'
    os::cmd::expect_success "docker rmi -f ${imageids}"
}

os::test::junit::declare_suite_start "end-to-end/core"

os::log::info "openshift version: `openshift version`"
os::log::info "oc version:        `oc version`"

# service dns entry is visible via master service
# find the IP of the master service by asking the API_HOST to verify DNS is running there
# might need to wait a bit to ensure the dns cache is primed
os::cmd::try_until_text "dig "@${API_HOST}" "kubernetes.default.svc.cluster.local." +short A | head -n 1" "([0-9]{1,3}\.){3}[0-9]{1,3}"
MASTER_SERVICE_IP="$(dig "@${API_HOST}" "kubernetes.default.svc.cluster.local." +short A | head -n 1)"
# find the IP of the master service again by asking the IP of the master service, to verify port 53 tcp/udp is routed by the service
os::cmd::expect_success_and_text "dig +tcp @${MASTER_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"
os::cmd::expect_success_and_text "dig +notcp @${MASTER_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"

# add e2e-user as a viewer for the default namespace so we can see infrastructure pieces appear
os::cmd::expect_success 'openshift admin policy add-role-to-user view e2e-user --namespace=default'

# pre-load some image streams and templates
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-stibuild.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/jenkins/application-template.json --namespace=openshift'

# create test project so that this shows up in the console
os::cmd::expect_success "openshift admin new-project test --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project docker --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project custom --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project cache --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"

echo "The console should be available at ${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}/console."
echo "Log in as 'e2e-user' to see the 'test' project."

# Client setup (log in as e2e-user and set 'test' as the default project)
# This is required to be able to push to the registry!
os::log::info "Logging in as a regular user (e2e-user:pass) with project 'test'..."
os::cmd::expect_success 'oc login -u e2e-user -p pass'
os::cmd::expect_success_and_text 'oc whoami' 'e2e-user'

os::log::info "Back to 'test' context with 'e2e-user' user"
os::cmd::expect_success 'oc login -u e2e-user'
os::cmd::expect_success 'oc project test'
os::cmd::expect_success 'oc whoami'

os::log::info "Running a CLI command in a container using the service account"
os::cmd::expect_success 'oc policy add-role-to-user view -z default'
os::cmd::try_until_success "oc sa get-token default"
oc run cli-with-token --attach --image="openshift/origin:${TAG}" --restart=Never -- cli status --loglevel=4 > "${LOG_DIR}/cli-with-token.log" 2>&1
os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token.log'" 'Using in-cluster configuration'
os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token.log'" 'In project test'
os::cmd::expect_success 'oc delete pod cli-with-token'
oc run cli-with-token-2 --attach --image="openshift/origin:${TAG}" --restart=Never -- cli whoami --loglevel=4 > "${LOG_DIR}/cli-with-token2.log" 2>&1
os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token2.log'" 'system:serviceaccount:test:default'
os::cmd::expect_success 'oc delete pod cli-with-token-2'
oc run kubectl-with-token --attach --image="openshift/origin:${TAG}" --restart=Never --command -- kubectl get pods --loglevel=4 > "${LOG_DIR}/kubectl-with-token.log" 2>&1
os::cmd::expect_success_and_text "cat '${LOG_DIR}/kubectl-with-token.log'" 'Using in-cluster configuration'
os::cmd::expect_success_and_text "cat '${LOG_DIR}/kubectl-with-token.log'" 'kubectl-with-token'

os::test::junit::declare_suite_end
unset VERBOSE
