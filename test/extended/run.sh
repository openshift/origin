#!/bin/bash

echo "[INFO] Applying Docker application config"
osc process -n docker -f examples/sample-app/application-template-dockerbuild.json \
  | osc create -n docker -f -
echo "[INFO] Invoking generic web hook to trigger new docker build using curl"
curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=docker && sleep 3

wait_for_build "docker"
wait_for_app "docker"

echo "[INFO] Applying Custom application config"
osc process -n custom -f examples/sample-app/application-template-custombuild.json \
  | osc create -n custom -f -
echo "[INFO] Invoking generic web hook to trigger new custom build using curl"
curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=custom && sleep 3

wait_for_build "custom"
wait_for_app "custom"
