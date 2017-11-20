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
  oc delete project project-bar
  exit 0
) &>/dev/null

login_kubeconfig="${ARTIFACT_DIR}/login.kubeconfig"
cp "${KUBECONFIG}" "${login_kubeconfig}"

os::test::junit::declare_suite_start "cmd/status"
# login and ensure no current projects exist
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
os::cmd::expect_success 'oc delete project --all'
os::cmd::try_until_text "oc get projects -o jsonpath='{.items}'" "^\[\]$"
os::cmd::expect_success 'oc logout'

# remove self-provisioner role from user and test login prompt before creating any projects
os::cmd::expect_success "oc adm policy remove-cluster-role-from-group self-provisioner system:authenticated:oauth --config='${login_kubeconfig}'"

# login as 'test-user'
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"

# make sure `oc status` re-uses the correct "no projects" message from `oc login` with no self-provisioner role
os::cmd::expect_success_and_text 'oc status' "You don't have any projects. Contact your system administrator to request a project"
os::cmd::expect_success_and_text 'oc status --all-namespaces' "Showing all projects on server"
# make sure standard login prompt is printed once self-provisioner status is restored
os::cmd::expect_success "oc logout"
os::cmd::expect_success "oc adm policy add-cluster-role-to-group self-provisioner system:authenticated:oauth --config='${login_kubeconfig}'"
os::cmd::expect_success_and_text "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything" "You don't have any projects. You can try to create a new project, by running"

# make sure `oc status` re-uses the correct "no projects" message from `oc login`
os::cmd::expect_success_and_text 'oc status' "You don't have any projects. You can try to create a new project, by running"
os::cmd::expect_success_and_text 'oc status --all-namespaces' "Showing all projects on server"
# make sure `oc status` does not re-use the "no projects" message from `oc login` if -n is specified
os::cmd::expect_failure_and_text 'oc status -n forbidden' 'Error from server \(Forbidden\): projects.project.openshift.io "forbidden" is forbidden: User "test-user" cannot get projects.project.openshift.io in the namespace "forbidden"'

# create a new project
os::cmd::expect_success "oc new-project project-bar --display-name='my project' --description='test project'"
os::cmd::expect_success_and_text "oc project" 'Using project "project-bar"'

# make sure `oc status` does not use "no projects" message if there is a project created
os::cmd::expect_success_and_text 'oc status' "In project my project \(project-bar\) on server"
os::cmd::expect_failure_and_text 'oc status -n forbidden' 'Error from server \(Forbidden\): projects.project.openshift.io "forbidden" is forbidden: User "test-user" cannot get projects.project.openshift.io in the namespace "forbidden"'

# create a second project
os::cmd::expect_success "oc new-project project-bar-2 --display-name='my project 2' --description='test project 2'"
os::cmd::expect_success_and_text "oc project" 'Using project "project-bar-2"'

# delete the current project `project-bar-2` and make sure `oc status` does not return the "no projects"
# message since `project-bar` still exists
os::cmd::expect_success_and_text "oc delete project project-bar-2" 'project "project-bar-2" deleted'
# the deletion is asynchronous and can take a while, so wait until we see the error
os::cmd::try_until_text "oc status" 'Error from server \(Forbidden\): projects.project.openshift.io "project-bar-2" is forbidden: User "test-user" cannot get projects.project.openshift.io in the namespace "project-bar-2"'

# delete "project-bar" and test that `oc status` still does not return the "no projects" message.
# Although we are deleting the last remaining project, the current context's namespace is still set
# to it, therefore `oc status` should simply return a forbidden error and not the "no projects" message
# until the next time the user logs in.
os::cmd::expect_success "oc project project-bar"
os::cmd::expect_success "oc delete project project-bar"
# the deletion is asynchronous and can take a while, so wait until we see the error
os::cmd::try_until_text "oc status" 'Error from server \(Forbidden\): projects.project.openshift.io "project-bar" is forbidden: User "test-user" cannot get projects.project.openshift.io in the namespace "project-bar"'
os::cmd::try_until_not_text "oc get projects" "project-bar"
os::cmd::try_until_not_text "oc get projects" "project-bar-2"
os::cmd::expect_success "oc logout"
os::cmd::expect_success_and_text "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything" "You don't have any projects. You can try to create a new project, by running"
os::cmd::expect_success_and_text 'oc status' "You don't have any projects. You can try to create a new project, by running"

# logout
os::cmd::expect_success "oc logout"

echo "status: ok"
os::test::junit::declare_suite_end
