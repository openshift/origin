#!/bin/bash -e

. shared.sh

curl \
  -H 'X-Broker-API-Version: 2.9' \
  -v \
  $curlargs \
  $endpoint/v2/catalog
