#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates,secrets,pods,jobs --all
  oc delete image v1-image
  oc delete group patch-group
  oc delete project test-project-admin
  oc delete oauthaccesstokens.oauth.openshift.io/DYGZDLucARCPIfUeKPhsgPfn0WBLR_9KdeREH0c9iod
  oc delete -f ${TEST_DATA}/multiport-service.yaml
  exit 0
) &>/dev/null

function escape_regex() {
  sed 's/[]\.|$(){}?+*^]/\\&/g' <<< "$*"
}

project="$( oc project -q )"

os::test::junit::declare_suite_start "cmd/basicresources"
# This test validates basic resource retrieval and command interaction

os::test::junit::declare_suite_start "cmd/basicresources/versionreporting"
# Test to make sure that we're reporting the correct version information from endpoints and the correct
# User-Agent information from our clients regardless of which resources they're trying to access
#os::build::version::get_vars
#os_git_regex="$( escape_regex "${OS_GIT_VERSION%%-*}" )"
#kube_git_regex="$( escape_regex "${KUBE_GIT_VERSION%%-*}" )"
#etcd_version="$(echo "${ETCD_GIT_VERSION}" | sed -E "s/\-.*//g" | sed -E "s/v//")"
#etcd_git_regex="$( escape_regex "${etcd_version%%-*}" )"
#os::cmd::expect_success_and_text 'oc version' "Client Version: .*GitVersion:\"${os_git_regex}"
#os::cmd::expect_success_and_text 'oc version' "Server Version: .*GitVersion:\"${kube_git_regex}"
#os::cmd::expect_success_and_text "curl -k '${API_SCHEME}://${API_HOST}:${API_PORT}/version'" "${kube_git_regex}"
#os::cmd::expect_success_and_text "curl -k '${API_SCHEME}://${API_HOST}:${API_PORT}/version'" "${OS_GIT_COMMIT}"

# variants I know I have to worry about
# 1. oc (kube and openshift resources)
# 2. oc adm (kube and openshift resources)

# example User-Agent: oc/v1.2.0 (linux/amd64) kubernetes/bc4550d
#os::cmd::expect_success_and_text 'oc get pods --loglevel=7  2>&1 | grep -A4 "pods" | grep User-Agent' "oc/${kube_git_regex} .* kubernetes/"
## example User-Agent: oc/v1.2.0 (linux/amd64) kubernetes/bc4550d
#os::cmd::expect_success_and_text 'oc get dc --loglevel=7  2>&1 | grep -A4 "deploymentconfig" | grep User-Agent' "oc/${kube_git_regex} .* kubernetes/"
## example User-Agent: oc/v1.1.3 (linux/amd64) openshift/b348c2f
#os::cmd::expect_success_and_text 'oc adm policy who-can get pods --loglevel=7  2>&1 | grep -A4 "localresourceaccessreviews" | grep User-Agent' "oc/${kube_git_regex} .* kubernetes/"
#echo "version reporting: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/pods"
os::cmd::expect_success 'oc get pods'
os::cmd::expect_success_and_text 'oc create -f ${TEST_DATA}/hello-openshift/hello-pod.json' 'pod/hello-openshift created'
os::cmd::expect_success 'oc describe pod hello-openshift'
os::cmd::expect_success 'oc delete pods hello-openshift --grace-period=0 --force'
echo "pods: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/expose"
# Expose service as a route
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-service.json'
os::cmd::expect_failure 'oc expose service frontend --create-external-load-balancer'
os::cmd::expect_failure 'oc expose service frontend --port=40 --type=NodePort'
os::cmd::expect_success 'oc expose service frontend --path=/test'
os::cmd::expect_success_and_text "oc get route.v1.route.openshift.io frontend --template='{{.spec.path}}'" "/test"
os::cmd::expect_success_and_text "oc get route.v1.route.openshift.io frontend --template='{{.spec.to.name}}'" "frontend"           # routes to correct service
os::cmd::expect_success_and_text "oc get route.v1.route.openshift.io frontend --template='{{.spec.port.targetPort}}'" ""
os::cmd::expect_success 'oc delete svc,route -l name=frontend'
# Test that external services are exposable
os::cmd::expect_success 'oc create -f ${TEST_DATA}/external-service.yaml'
os::cmd::expect_success 'oc expose svc/external'
os::cmd::expect_success_and_text 'oc get route external' 'external'
os::cmd::expect_success 'oc delete route external'
os::cmd::expect_success 'oc delete svc external'
# Expose multiport service and verify we set a port in the route
os::cmd::expect_success 'oc create -f ${TEST_DATA}/multiport-service.yaml'
os::cmd::expect_success 'oc expose svc/frontend --name route-with-set-port'
os::cmd::expect_success_and_text "oc get route route-with-set-port --template='{{.spec.port.targetPort}}'" "web"
echo "expose: ok"
os::test::junit::declare_suite_end

# Test OAuth access token describer
os::cmd::expect_success 'oc create -f ${TEST_DATA}/oauthaccesstoken.yaml'
os::cmd::expect_success_and_text "oc describe oauthaccesstoken sha256~efaca6fab897953ffcb4f857eb5cbf2cf3a4c33f1314b4922656303426b1cfc9" "efaca6fab897953ffcb4f857eb5cbf2cf3a4c33f1314b4922656303426b1cfc9"
echo "OAuth descriptor: ok"

os::cmd::expect_success 'oc delete all --all'

os::test::junit::declare_suite_start "cmd/basicresources/projectadmin"
# switch to test user to be sure that default project admin policy works properly
temp_config="$(mktemp -d)/tempconfig"
os::cmd::expect_success "oc config view --raw > '${temp_config}'"
export KUBECONFIG="${temp_config}"
#os::cmd::expect_success 'oc policy add-role-to-user admin project-admin'
#os::cmd::expect_success 'oc login -u project-admin -p anything'
#os::cmd::expect_success 'oc new-project test-project-admin'
#os::cmd::try_until_success "oc project test-project-admin"

os::cmd::expect_success 'oc create deploymentconfig --image=image-registry.openshift-image-registry.svc:5000/openshift/tools:latest test'
os::cmd::expect_success 'oc run --image=image-registry.openshift-image-registry.svc:5000/openshift/tools:latest --restart=Never test3'
os::cmd::expect_success 'oc create job --image=image-registry.openshift-image-registry.svc:5000/openshift/tools:latest test4'
os::cmd::expect_success 'oc delete dc/test pod/test3 job/test4'

os::cmd::expect_success_and_text 'oc create deploymentconfig --dry-run foo --image=bar -o name'               'deploymentconfig.apps.openshift.io/foo'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o name --restart=Never'                'pod/foo'
os::cmd::expect_success_and_text 'oc create job --dry-run foo --image=bar -o name'              'job.batch/foo'
os::cmd::expect_success_and_text 'oc create deploymentconfig --dry-run foo --image=bar -o name' 'deploymentconfig.apps.openshift.io/foo'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o name'          'pod/foo'

os::cmd::expect_success 'oc process -f ${TEST_DATA}/application-template-stibuild.json -l name=mytemplate | oc create -f -'
os::cmd::expect_success 'oc delete all -l name=mytemplate'
#os::cmd::expect_success 'oc new-app https://github.com/openshift/ruby-hello-world'
#os::cmd::expect_success 'oc get dc/ruby-hello-world'

#os::cmd::expect_success_and_text "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'" '1'
#patch='{"spec": {"replicas": 2}}'
#os::cmd::expect_success "oc patch dc/ruby-hello-world -p '${patch}'"
#os::cmd::expect_success_and_text "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'" '2'
#
#os::cmd::expect_success 'oc delete all -l app=ruby-hello-world'
#os::cmd::expect_failure 'oc get dc/ruby-hello-world'
echo "delete all: ok"
os::test::junit::declare_suite_end

# service accounts should not be allowed to request new projects
# TODO re-enable once we can use tokens instead of certs
#os::cmd::expect_failure_and_text "oc new-project --token='$( oc sa get-token builder )' will-fail" 'Error from server \(Forbidden\): You may not request a new project via this API.'

os::test::junit::declare_suite_end
