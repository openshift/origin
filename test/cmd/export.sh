#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates the export command

oc new-app -f examples/sample-app/application-template-stibuild.json --name=sample

oc export all --all-namespaces

# make sure the deprecated flag doesn't fail
oc export all --all

[ "$(oc export svc -t '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' | wc -l)" -ne 0 ]
[ "$(oc export svc --as-template=template | grep 'kind: Template')" ]
[ ! "$(oc export svc | grep 'clusterIP')" ]
[ ! "$(oc export svc --exact | grep 'clusterIP: ""')" ]
[ ! "$(oc export svc --raw | grep 'clusterIP: ""')" ]
[ ! "$(oc export svc --raw --exact)" ]
[ ! "$(oc export svc -l a=b)" ] # return error if no items match selector
[ "$(oc export svc -l a=b 2>&1 | grep 'no resources found')" ]
[ "$(oc export svc -l app=sample)" ]
[ "$(oc export -f examples/sample-app/application-template-stibuild.json --raw --output-version=v1 | grep 'apiVersion: v1')" ]
echo "export: ok"
