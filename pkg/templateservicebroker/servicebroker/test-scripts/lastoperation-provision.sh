#!/bin/bash -e

. shared.sh

curl \
    -H 'X-Broker-API-Version: 2.9' \
  -H "X-Broker-API-Originating-Identity: Kubernetes $(echo -ne "{\"username\": \"$requesterUsername\"}" | base64)" \
  -v \
  $curlargs \
  $endpoint/v2/service_instances/$instanceUUID/last_operation'?operation=provisioning'
