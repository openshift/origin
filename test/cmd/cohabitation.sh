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


os::test::junit::declare_suite_start "cmd/cohabitation/rc-rs"

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

os::cmd::expect_success 'oc scale rc/nginx --replicas=1'
os::cmd::expect_success 'oc scale rc/frontend --replicas=1'
os::cmd::expect_success 'oc scale rs/nginx --replicas=1'
os::cmd::expect_success 'oc scale rs/frontend --replicas=1'

echo "rs<>rs: ok"
os::test::junit::declare_suite_end



os::test::junit::declare_suite_start "cmd/cohabitation/d-dc"

os::cmd::expect_success 'oc create -f test/extended/testdata/deployment-simple.yaml -n cmd-cohabitation'
os::cmd::expect_success 'oc create -f vendor/k8s.io/kubernetes/docs/user-guide/deployment.yaml -n cmd-cohabitation'

os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs" '"kind": "DeploymentConfigList"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs" '"kind": "DeploymentList"'
os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments" '"kind": "DeploymentList"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments" '"kind": "DeploymentConfigList"'

os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs/deployment-simple" '"kind": "DeploymentConfig"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs/deployment-simple" '"kind": "Deployment"'
os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments/deployment-simple" '"kind": "Deployment"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments/deployment-simple" '"kind": "DeploymentConfig"'

os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs/nginx-deployment" '"kind": "DeploymentConfig"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs/nginx-deployment" '"kind": "Deployment"'
os::cmd::expect_success_and_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments/nginx-deployment" '"kind": "Deployment"'
os::cmd::expect_success_and_not_text "curl -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments/nginx-deployment" '"kind": "DeploymentConfig"'

os::cmd::expect_failure_and_text "curl -k --max-time 1 -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs?watch=true" '"kind":"DeploymentConfig"'
os::cmd::expect_failure_and_not_text "curl --max-time 1 -k -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-cohabitation/deploymentconfigs?watch=true" '"kind":"Deployment"'
os::cmd::expect_failure_and_text "curl -k --max-time 1 -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments?watch=true" '"kind":"Deployment"'
os::cmd::expect_failure_and_not_text "curl -k --max-time 1 -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/apis/extensions/v1beta1/namespaces/cmd-cohabitation/deployments?watch=true" '"kind":"DeploymentConfig"'

os::cmd::expect_success 'oc scale dc/deployment-simple --replicas=1'
os::cmd::expect_failure_and_text 'oc scale dc/nginx-deployment --replicas=1' "wrong native type, no cross type updates allowed"
os::cmd::expect_success 'oc scale deployment/nginx-deployment --replicas=1'
os::cmd::expect_failure_and_text 'oc scale deployment/deployment-simple --replicas=1' "wrong native type, no cross type updates allowed"
os::cmd::expect_success 'oc rollout pause deployment/nginx-deployment'
os::cmd::expect_failure_and_text 'oc rollout pause deployment/deployment-simple' "wrong native type, no cross type updates allowed"

echo "d<>dc: ok"
os::test::junit::declare_suite_end


echo "cohabitation: ok"
os::test::junit::declare_suite_end
