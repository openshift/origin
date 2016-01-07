#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates basic resource retrieval and command interaction

os::cmd::expect_success_and_text 'oc types' 'Deployment Configuration'
os::cmd::expect_failure_and_text 'oc get' 'deploymentconfig'

# Test resource builder filtering of files with expected extensions inside directories, and individual files without expected extensions
os::cmd::expect_success 'oc create -f test/fixtures/resource-builder/directory -f test/fixtures/resource-builder/json-no-extension -f test/fixtures/resource-builder/yml-no-extension'
# Explicitly specified extensionless files
os::cmd::expect_success 'oc get secret json-no-extension yml-no-extension'
# Scanned files with extensions inside directories
os::cmd::expect_success 'oc get secret json-with-extension yml-with-extension'
# Ensure extensionless files inside directories are not processed by resource-builder
os::cmd::expect_failure_and_text 'oc get secret json-no-extension-in-directory' 'not found'
echo "resource-builder: ok"

os::cmd::expect_success 'oc get pods --match-server-version'
os::cmd::expect_success_and_text 'oc create -f examples/hello-openshift/hello-pod.json' 'pod "hello-openshift" created'
os::cmd::expect_success 'oc describe pod hello-openshift'
os::cmd::expect_success 'oc delete pods hello-openshift'
echo "pods: ok"

os::cmd::expect_success_and_text 'oc create -f examples/hello-openshift/hello-pod.json -o name' 'pod/hello-openshift'
os::cmd::try_until_success 'oc label pod/hello-openshift acustom=label' # can race against scheduling and status updates
os::cmd::expect_success_and_text 'oc describe pod/hello-openshift' 'acustom=label'
os::cmd::try_until_success 'oc annotate pod/hello-openshift foo=bar' # can race against scheduling and status updates
os::cmd::expect_success_and_text 'oc get -o yaml pod/hello-openshift' 'foo: bar'
os::cmd::expect_success 'oc delete pods -l acustom=label'
os::cmd::expect_failure 'oc get pod/hello-openshift'
echo "label: ok"

os::cmd::expect_success 'oc get services'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-service.json'
os::cmd::expect_success 'oc delete services frontend'
echo "services: ok"

os::cmd::expect_success 'oc create -f test/fixtures/mixed-api-versions.yaml'
os::cmd::expect_success 'oc get    -f test/fixtures/mixed-api-versions.yaml -o yaml'
os::cmd::expect_success 'oc delete -f test/fixtures/mixed-api-versions.yaml'
echo "list version conversion: ok"

os::cmd::expect_success 'oc get nodes'
(
  # subshell so we can unset kubeconfig
  cfg="${KUBECONFIG}"
  unset KUBECONFIG
  os::cmd::expect_success 'kubectl get nodes --kubeconfig="${cfg}"'
)
echo "nodes: ok"

os::cmd::expect_success 'oc get routes'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-route.json'
os::cmd::expect_success 'oc delete routes testroute'
echo "routes: ok"

# Expose service as a route
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-service.json'
os::cmd::expect_failure 'oc expose service frontend --create-external-load-balancer'
os::cmd::expect_failure 'oc expose service frontend --port=40 --type=NodePort'
os::cmd::expect_success 'oc expose service frontend'
os::cmd::expect_success_and_text "oc get route frontend --output-version=v1 --template='{{.spec.to.name}}'" "frontend"           # routes to correct service
os::cmd::expect_success_and_text "oc get route frontend --output-version=v1 --template='{{.spec.port.targetPort}}'" "<no value>" # no target port for services with unnamed ports
os::cmd::expect_success 'oc delete svc,route -l name=frontend'
# Test that external services are exposable
os::cmd::expect_success 'oc create -f test/fixtures/external-service.yaml'
os::cmd::expect_success 'oc expose svc/external'
os::cmd::expect_success_and_text 'oc get route external' 'external=service'
os::cmd::expect_success 'oc delete route external'
os::cmd::expect_success 'oc delete svc external'
# Expose multiport service and verify we set a port in the route
os::cmd::expect_success 'oc create -f test/fixtures/multiport-service.yaml'
os::cmd::expect_success 'oc expose svc/frontend --name route-with-set-port'
os::cmd::expect_success_and_text "oc get route route-with-set-port --template='{{.spec.port.targetPort}}' --output-version=v1" "web"
echo "expose: ok"

os::cmd::expect_success 'oc delete all --all'

# switch to test user to be sure that default project admin policy works properly
project=$(oc project -q)
os::cmd::expect_success 'oc policy add-role-to-user admin test-user'
os::cmd::expect_success 'oc login -u test-user -p anything'
os::cmd::try_until_success 'oc project ${project}'

os::cmd::expect_success 'oc run --image=openshift/hello-openshift test'
os::cmd::expect_success 'oc run --image=openshift/hello-openshift --generator=run-controller/v1 test2'
os::cmd::expect_success 'oc run --image=openshift/hello-openshift --restart=Never test3'
os::cmd::expect_success 'oc delete dc/test rc/test2 pod/test3'

os::cmd::expect_success 'oc process -f examples/sample-app/application-template-stibuild.json -l name=mytemplate | oc create -f -'
os::cmd::expect_success 'oc delete all -l name=mytemplate'
os::cmd::expect_success 'oc new-app https://github.com/openshift/ruby-hello-world'
os::cmd::expect_success 'oc get dc/ruby-hello-world'

os::cmd::expect_success_and_text "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'" '1'
patch='{"spec": {"replicas": 2}}'
os::cmd::expect_success "oc patch dc/ruby-hello-world -p '${patch}'"
os::cmd::expect_success_and_text "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'" '2'

os::cmd::expect_success 'oc delete all -l app=ruby-hello-world'
os::cmd::expect_failure 'oc get dc/ruby-hello-world'
echo "delete all: ok"

