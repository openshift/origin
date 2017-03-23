#!/bin/bash -e

. shared.sh

curl \
  -X DELETE \
  -H 'X-Broker-API-Version: 2.9' \
  -v \
  $curlargs \
  $endpoint/v2/service_instances/$instanceUUID/service_bindings/$bindingUUID
