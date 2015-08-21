#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates basic resource retrieval and command interaction

# Test resource builder filtering of files with expected extensions inside directories, and individual files without expected extensions
[ "$(oc create -f test/resource-builder/directory -f test/resource-builder/json-no-extension -f test/resource-builder/yml-no-extension 2>&1)" ]
# Explicitly specified extensionless files
oc get secret json-no-extension yml-no-extension
# Scanned files with extensions inside directories
oc get secret json-with-extension yml-with-extension
# Ensure extensionless files inside directories are not processed by resource-builder
[ "$(oc get secret json-no-extension-in-directory 2>&1 | grep 'not found')" ]
echo "resource-builder: ok"

oc get pods --match-server-version
oc create -f examples/hello-openshift/hello-pod.json
oc describe pod hello-openshift
oc delete pods hello-openshift
echo "pods: ok"

oc create -f examples/hello-openshift/hello-pod.json
tryuntil oc label pod/hello-openshift acustom=label # can race against scheduling and status updates
[ "$(oc describe pod/hello-openshift | grep 'acustom=label')" ]
oc delete pods -l acustom=label
[ ! "$(oc get pod/hello-openshift)" ]
echo "label: ok"

oc get services
oc create -f test/integration/fixtures/test-service.json
oc delete services frontend
echo "services: ok"

oc get nodes
echo "nodes: ok"

oc get routes
oc create -f test/integration/fixtures/test-route.json
oc delete routes testroute
echo "routes: ok"

# Expose service as a route
oc create -f test/integration/fixtures/test-service.json
[ ! "$(oc expose service frontend --create-external-load-balancer)" ]
[ ! "$(oc expose service frontend --port=40 --type=NodePort)" ]
oc expose service frontend
[ "$(oc get route frontend | grep 'name=frontend')" ]
oc delete svc,route -l name=frontend
echo "expose: ok"

oc delete all --all

oc process -f examples/sample-app/application-template-stibuild.json -l name=mytemplate | oc create -f -
oc delete all -l name=mytemplate
oc new-app https://github.com/openshift/ruby-hello-world
[ "$(oc get dc/ruby-hello-world)" ]
oc delete all -l app=ruby-hello-world
[ ! "$(oc get dc/ruby-hello-world)" ]
echo "delete all: ok"

