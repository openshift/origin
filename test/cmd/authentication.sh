#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

project="$( oc project -q )"
if [[ "${project}" == "default" ]]; then
  echo "Test must be run from a non-default namespace"
  exit 1
fi

# Cleanup cluster resources created by this test
(
  set +e
  oc delete oauthaccesstokens --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/authentication"

# Logging in prints useful messages
os::test::junit::declare_suite_start "cmd/authentication/existing-credentials"
os::cmd::expect_success_and_text 'oc login -u user1 -p pw' 'Login successful'
os::cmd::expect_success_and_text 'oc login -u user2 -p pw' 'Login successful'
# Switching to another user using existing credentials informs you
os::cmd::expect_success_and_text 'oc login -u user1'       'Logged into ".*" as "user1" using existing credentials'
# Completing a login as the same user using existing credentials informs you
os::cmd::expect_success_and_text 'oc login -u user1'       'Logged into ".*" as "user1" using existing credentials'
# Return to the system:admin user
os::cmd::expect_success "oc login -u system:admin -n '${project}'"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/authentication/scopedtokens"
os::cmd::expect_success 'oadm policy add-role-to-user admin scoped-user'

# initialize the user object
os::cmd::expect_success 'oc login -u scoped-user -p asdf'
os::cmd::expect_success 'oc login -u system:admin'
username="$(oc get user/scoped-user -o jsonpath='{.metadata.name}')"
useruid="$(oc get user/scoped-user -o jsonpath='{.metadata.uid}')"
os::cmd::expect_success_and_text "oc policy can-i --list -n '${project}' --as=scoped-user" 'get.*pods'

whoamitoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=whoami SCOPE=user:info USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "oc get user/~ --token='${whoamitoken}'" "${username}"
os::cmd::expect_failure_and_text "oc get pods --token='${whoamitoken}' -n '${project}'" "prevent this action; User \"scoped-user\" cannot list pods in project \"${project}\""

listprojecttoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=listproject SCOPE=user:list-scoped-projects USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
# this token doesn't have rights to see any projects even though it can hit the list endpoint, so an empty list is correct
# we'll add another scope that allows listing all known projects even if this token has no other powers in them.
os::cmd::expect_success_and_not_text "oc get projects --token='${listprojecttoken}'" "${project}"
os::cmd::expect_failure_and_text "oc get user/~ --token='${listprojecttoken}'" 'prevent this action; User "scoped-user" cannot get users at the cluster scope'
os::cmd::expect_failure_and_text "oc get pods --token='${listprojecttoken}' -n '${project}'" "prevent this action; User \"scoped-user\" cannot list pods in project \"${project}\""

listprojecttoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=listallprojects SCOPE=user:list-projects USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "oc get projects --token='${listprojecttoken}'" "${project}"

adminnonescalatingpowerstoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=admin SCOPE=role:admin:* USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_failure_and_text "oc get user/~ --token='${adminnonescalatingpowerstoken}'" 'prevent this action; User "scoped-user" cannot get users at the cluster scope'
os::cmd::expect_failure_and_text "oc get secrets --token='${adminnonescalatingpowerstoken}' -n '${project}'" "prevent this action; User \"scoped-user\" cannot list secrets in project \"${project}\""
os::cmd::expect_success_and_text "oc get 'projects/${project}' --token='${adminnonescalatingpowerstoken}' -n '${project}'" "${project}"

allescalatingpowerstoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=clusteradmin SCOPE='role:cluster-admin:*:!' USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "oc get user/~ --token='${allescalatingpowerstoken}'" "${username}"
os::cmd::expect_success "oc get secrets --token='${allescalatingpowerstoken}' -n '${project}'"
# scopes allow it, but authorization doesn't
os::cmd::try_until_failure "oc get secrets --token='${allescalatingpowerstoken}' -n default"
os::cmd::expect_failure_and_text "oc get secrets --token='${allescalatingpowerstoken}' -n default" 'cannot list secrets in project'
os::cmd::expect_success_and_text "oc get projects --token='${allescalatingpowerstoken}'" "${project}"
os::cmd::expect_success_and_text "oc policy can-i --list --token='${allescalatingpowerstoken}' -n '${project}'" 'get.*pods'

accesstoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=access SCOPE=user:check-access USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "curl -k -XPOST -H 'Content-Type: application/json' -H 'Authorization: Bearer ${accesstoken}' '${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/${project}/localsubjectaccessreviews' -d @${OS_ROOT}/test/testdata/authentication/localsubjectaccessreview.json" '"kind": "SubjectAccessReviewResponse"'
os::cmd::expect_success_and_text "oc policy can-i create pods --token='${accesstoken}' -n '${project}' --ignore-scopes" 'yes'
os::cmd::expect_success_and_text "oc policy can-i create pods --token='${accesstoken}' -n '${project}'" 'no'
os::cmd::expect_success_and_text "oc policy can-i create subjectaccessreviews --token='${accesstoken}' -n '${project}'" 'no'
os::cmd::expect_success_and_text "oc policy can-i create subjectaccessreviews --token='${accesstoken}' -n '${project}' --ignore-scopes" 'yes'
os::cmd::expect_success_and_text "oc policy can-i create pods --token='${accesstoken}' -n '${project}' --scopes='role:admin:*'" 'yes'
os::cmd::expect_success_and_text "oc policy can-i --list --token='${accesstoken}' -n '${project}' --scopes='role:admin:*'" 'get.*pods'
os::cmd::expect_success_and_not_text "oc policy can-i --list --token='${accesstoken}' -n '${project}'" 'get.*pods'


os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
