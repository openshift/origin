#!/bin/bash -e

. shared.sh

serviceUUID=${serviceUUID-$(oc get template cakephp-mysql-example -n openshift -o template --template '{{.metadata.uid}}')}

req="{
  \"plan_id\": \"$planUUID\",
  \"service_id\": \"$serviceUUID\"
}"

curl \
  -X PUT \
  -H "$apiVersion" \
  -H 'Content-Type: application/json' \
  -H "X-Broker-API-Originating-Identity: Kubernetes $(echo -ne "{\"username\": \"$requesterUsername\", \"groups\": [\"system:authenticated\"]}" | base64 -w 100)" \
  -d "$req" \
  -v \
  $curlargs \
  $endpoint/v2/service_instances/$instanceUUID/service_bindings/$bindingUUID
