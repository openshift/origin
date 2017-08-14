#!/bin/bash -e


apiVersion="X-Broker-API-Version: 2.12"
instanceUUID=${instanceUUID-a71f7ab8-e448-4826-8f05-32a185222dd7}
planUUID=${planUUID-7ae2bd88-9b8f-4a17-8014-41a5465c9e71}
bindingUUID=${bindingUUID-dde0226b-ff95-4f9d-af51-2e9ec06b1f02}

svcIP=$(oc get svc apiserver -n openshift-template-service-broker --template '{{.spec.clusterIP}}' -o template)
endpoint=${endpoint-https://$svcIP/brokers/template.openshift.io}
curlargs=${curlargs--k}
namespace=${namespace-myproject}
requesterUsername=${requesterUsername-$(oc whoami)}
