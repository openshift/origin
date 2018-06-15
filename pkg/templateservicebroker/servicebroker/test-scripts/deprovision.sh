#!/bin/bash -e

. shared.sh

curl \
  -X DELETE \
  -H "$apiVersion" \
  -H "X-Broker-API-Originating-Identity: Kubernetes $(echo -ne "{\"username\": \"$requesterUsername\", \"groups\": [\"system:authenticated\"]}" | base64 -w 100)" \
  -v \
  $curlargs \
  $endpoint/v2/service_instances/$instanceUUID'?accepts_incomplete=true'
