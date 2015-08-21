#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# This test validates the export command

oc new-app -f examples/sample-app/application-template-stibuild.json --name=sample

oc export all --all --all-namespaces --exact

[ "$(oc export svc --all -t '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' | wc -l)" -ne 0 ]
[ "$(oc export svc --all --as-template=template | grep 'kind: Template')" ]
[ ! "$(oc export svc --all | grep 'clusterIP')" ]
[ ! "$(oc export svc --all --exact | grep 'clusterIP: ""')" ]
[ ! "$(oc export svc --all --raw | grep 'clusterIP: ""')" ]
[ ! "$(oc export svc --all --raw --exact)" ]
[ ! "$(oc export svc -l a=b)" ] # return error if no items match selector
[ "$(oc export svc -l a=b 2>&1 | grep 'no resources found')" ]
[ "$(oc export svc -l app=sample)" ]
[ "$(oc export -f examples/sample-app/application-template-stibuild.json --raw --output-version=v1 | grep 'apiVersion: v1')" ]
echo "export: ok"
