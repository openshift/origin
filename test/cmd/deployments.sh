#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates deployments

oc get deploymentConfigs
oc get dc
oc create -f test/integration/fixtures/test-deployment-config.json
oc describe deploymentConfigs test-deployment-config
[ "$(oc env dc/test-deployment-config --list | grep TEST=value)" ]
[ ! "$(oc env dc/test-deployment-config TEST- --list | grep TEST=value)" ]
[ "$(oc env dc/test-deployment-config TEST=foo --list | grep TEST=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo --list | grep TEST=value)" ]
[ ! "$(oc env dc/test-deployment-config OTHER=foo -c 'ruby' --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -c 'ruby*'   --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -c '*hello*' --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -c '*world'  --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -o yaml | grep "name: OTHER")" ]
[ "$(echo "OTHER=foo" | oc env dc/test-deployment-config -e - --list | grep OTHER=foo)" ]
[ ! "$(echo "#OTHER=foo" | oc env dc/test-deployment-config -e - --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config TEST=bar OTHER=baz BAR-)" ]

oc deploy test-deployment-config
oc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

oc delete all --all

oc create -f test/integration/fixtures/test-deployment-config.json
oc deploy test-deployment-config --latest
tryuntil oc get rc/test-deployment-config-1
# scale rc via deployment configuration
oc scale dc test-deployment-config --replicas=1
# scale directly
oc scale rc test-deployment-config-1 --replicas=5
oc delete all --all
echo "scale: ok"

oc delete all --all

oc process -f examples/sample-app/application-template-dockerbuild.json -l app=dockerbuild | oc create -f -
tryuntil oc get rc/database-1

oc rollback database --to-version=1 -o=yaml
oc rollback dc/database --to-version=1 -o=yaml
oc rollback dc/database --to-version=1 --dry-run
oc rollback database-1 -o=yaml
oc rollback rc/database-1 -o=yaml
# should fail because there's no previous deployment
[ ! "$(oc rollback database -o yaml)" ]
echo "rollback: ok"

oc get dc/database
oc stop dc/database
[ ! "$(oc get dc/database)" ]
[ ! "$(oc get rc/database-1)" ]
echo "stop: ok"
