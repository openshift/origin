#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete project project-foo
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/login"
# This test validates login functionality for the client
# we want this test to run without $KUBECONFIG or $KUBERNETES_MASTER as it tests that functionality
# `oc` will use in-cluster config if KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT
# are set, as well as /var/run/secrets/kubernetes.io/serviceaccount/token exists. we
# therefore can be sure that we are picking up no client configuration if we unset these variables
login_kubeconfig="${ARTIFACT_DIR}/login.kubeconfig"
cp "${KUBECONFIG}" "${login_kubeconfig}"
unset KUBECONFIG
unset KUBERNETES_MASTER
# test client not configured
os::cmd::expect_failure_and_text "env -u KUBERNETES_SERVICE_HOST oc get services" 'Missing or incomplete configuration info.  Please login'
unused_port="33333"
# setting env bypasses the not configured message
os::cmd::expect_failure_and_text "env -u KUBERNETES_SERVICE_HOST KUBERNETES_MASTER=http://${API_HOST}:${unused_port} oc get services" 'did you specify the right host or port'
# setting --server bypasses the not configured message
os::cmd::expect_failure_and_text "env -u KUBERNETES_SERVICE_HOST oc get services --server=http://${API_HOST}:${unused_port}" 'did you specify the right host or port'

# Set KUBERNETES_MASTER for oc from now on
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Set certificates for oc from now on
if [[ "${API_SCHEME}" == "https" ]]; then
    # test bad certificate
    os::cmd::expect_failure_and_text "env -u KUBERNETES_SERVICE_HOST oc get services" 'certificate signed by unknown authority'
fi

# remove self-provisioner role from user and test login prompt before creating any projects
os::cmd::expect_success "oc adm policy remove-cluster-role-from-group self-provisioner system:authenticated:oauth --config='${login_kubeconfig}'"
os::cmd::expect_success_and_text "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything" "You don't have any projects. Contact your system administrator to request a project"
# make sure standard login prompt is printed once self-provisioner status is restored
os::cmd::expect_success "oc adm policy add-cluster-role-to-group self-provisioner system:authenticated:oauth --config='${login_kubeconfig}'"
os::cmd::expect_success_and_text "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything" "You don't have any projects. You can try to create a new project, by running"
# make sure `oc login` fails with unauthorized error
os::cmd::expect_failure_and_text 'oc login <<< \n' 'Login failed \(401 Unauthorized\)'
os::cmd::expect_success 'oc logout'
echo "login and status messages: ok"

# login and logout tests
# bad token should error
os::cmd::expect_failure_and_text "oc login ${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' --token=badvalue" 'The token provided is invalid or expired'
# --token and --username are mutually exclusive
os::cmd::expect_failure_and_text "oc login ${KUBERNETES_MASTER} -u test-user --token=tmp --insecure-skip-tls-verify" 'mutually exclusive'
# must only accept one arg (server)
os::cmd::expect_failure_and_text "oc login https://server1 https://server2.com" 'Only the server URL may be specified'
# logs in with a valid certificate authority
os::cmd::expect_success "oc login ${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
os::cmd::expect_success_and_text "cat ${HOME}/.kube/config" "v1"
os::cmd::expect_success 'oc logout'
# logs in skipping certificate check
os::cmd::expect_success "oc login ${KUBERNETES_MASTER} --insecure-skip-tls-verify -u test-user -p anything"
# logs in by an existing and valid token
temp_token="$(oc config view -o template --template='{{range .users}}{{ index .user.token }}{{end}}')"
os::cmd::expect_success_and_text "oc login --token=${temp_token}" 'using the token provided'
os::cmd::expect_success 'oc logout'
# properly parse server port
os::cmd::expect_failure_and_text 'oc login https://server1:844333' 'Not a valid port'
# properly handle trailing slash
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
# create a new project
os::cmd::expect_success "oc new-project project-foo --display-name='my project' --description='boring project description'"
os::cmd::expect_success_and_text "oc project" 'Using project "project-foo"'
# new user should get default context
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u new-and-unknown-user -p anything"
os::cmd::expect_success_and_text 'oc config view' "current-context.+/${API_HOST}:${API_PORT}/new-and-unknown-user"
# denies access after logging out
os::cmd::expect_success 'oc logout'
os::cmd::expect_failure_and_text 'oc get pods' '"system:anonymous" cannot list pods'

# make sure we report an error if the config file we pass is not writable
# Does not work inside of a container, determine why and reenable
# os::cmd::expect_failure_and_text "oc login '${KUBERNETES_MASTER}' -u test -p test '--config=${templocation}/file' --insecure-skip-tls-verify" 'KUBECONFIG is set to a file that cannot be created or modified'
echo "login warnings: ok"

# login and create serviceaccount and test login and logout with a service account token
os::cmd::expect_success "oc login ${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
os::cmd::expect_success_and_text "oc create sa testserviceaccount" "serviceaccount \"testserviceaccount\" created"
os::cmd::try_until_success "oc sa get-token testserviceaccount"
os::cmd::expect_success_and_text "oc login --token=$(oc sa get-token testserviceaccount)" "system:serviceaccount:project-foo:testserviceaccount"
# attempt to logout successfully
os::cmd::expect_success_and_text "oc logout" "Logged \"system:serviceaccount:project-foo:testserviceaccount\" out"
# verify that the token is no longer present in our local config
os::cmd::expect_failure_and_text "oc whoami" "User \"system:anonymous\" cannot get users"

# log in and set project to use from now on
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
os::cmd::expect_success 'oc get projects'
os::cmd::expect_success 'oc project project-foo'
os::cmd::expect_success_and_text 'oc config view' "current-context.+project-foo/${API_HOST}:${API_PORT}/test-user"
os::cmd::expect_success_and_text 'oc whoami' 'test-user'
os::cmd::expect_success_and_text "oc whoami --config='${login_kubeconfig}'" 'system:admin'
os::cmd::expect_success_and_text 'oc whoami -t' '.'
os::cmd::expect_success_and_text 'oc whoami -c' '.'

# test config files from the --config flag
os::cmd::expect_success "oc get services --config='${login_kubeconfig}'"
# test config files from env vars
os::cmd::expect_success "KUBECONFIG='${login_kubeconfig}' oc get services"
os::test::junit::declare_suite_end
