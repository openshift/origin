#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/cohabitation"

# integration tests ensure that the go client can list and watch and ensure that etcd is correct
# this test ensures that the bytes coming back from the API have the correct type

os::cmd::expect_success 'oadm policy add-role-to-user admin cohabiting-user -n cmd-cohabitation'
os::cmd::try_until_text 'oadm policy who-can create replicationcontrollers' 'cohabiting-user'
os::cmd::expect_success 'oc login -u cohabiting-user -p asdf'
accesstoken=$(oc whoami -t)
os::cmd::expect_success 'oc login -u system:admin'

os::cmd::expect_success 'oc create -f vendor/k8s.io/kubernetes/docs/user-guide/replication.yaml -n cmd-cohabitation'
os::cmd::expect_success 'oc create -f vendor/k8s.io/kubernetes/docs/user-guide/replicaset/frontend.yaml -n cmd-cohabitation'

os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers" '"kind": "ReplicationControllerList"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers" '"kind": "ReplicaSetList"'
os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets" '"kind": "ReplicaSetList"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets" '"kind": "ReplicationControllerList"'

os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers/frontend" '"kind": "ReplicationController"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers/frontend" '"kind": "ReplicaSet"'
os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets/frontend" '"kind": "ReplicaSet"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets/frontend" '"kind": "ReplicationController"'

os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers/nginx" '"kind": "ReplicationController"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers/nginx" '"kind": "ReplicaSet"'
os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets/nginx" '"kind": "ReplicaSet"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets/nginx" '"kind": "ReplicationController"'

os::cmd::expect_failure_and_text "curl -k --max-time 1 -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers?watch=true" '"kind":"ReplicationController"'
os::cmd::expect_failure_and_not_text "curl --max-time 1 -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/namespaces/cmd-cohabitation/replicationcontrollers?watch=true" '"kind":"ReplicaSet"'
os::cmd::expect_failure_and_text "curl -k --max-time 1 -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets?watch=true" '"kind":"ReplicaSet"'
os::cmd::expect_failure_and_not_text "curl -k --max-time 1 -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/replicasets?watch=true" '"kind":"ReplicationController"'


echo "cohabitation: ok"
os::test::junit::declare_suite_end
