#!/bin/bash

# OpenShift binary
openshift="../../_output/go/bin/openshift"

# Cleanup
function cleanup() {
  rm -rf openshift.local.*
  { pkill -TERM -P $$; } &>/dev/null || :
}
cleanup
trap 'cleanup' EXIT

# OPTIONAL: kill all Docker containers before starting
# docker kill `docker ps --no-trunc -q`

echo "Pre-pulling images:"
./pullimages.sh
echo

# Start the OpenShift all-in-one server
# (starts a kubernetes master and minion as well as providing the origin REST api)
echo "Launching OpenShift all-in-one server..."
mkdir -p logs
$openshift start &> logs/openshift.log &

echo -n "Waiting for OpenShift to become responsive.."
while true; do 
  curl http://localhost:8080/ &>/dev/null && echo -e "\n" && break
  echo -n "."
  sleep 0.5
done

sleep 5

# Deploy the private Docker registry config
echo "Deploying private Docker registry..."
$openshift kube apply -c registry-config.json

echo -n "Waiting for Docker registry service to start.."
while true; do
  case "$($openshift kube list services)" in
    *registryPod*) echo -e "\n" && break;;
    *) echo -n "."
  esac
  sleep 0.5
done

echo -n "Waiting for Docker registry pod to start.."
while true; do
  case "$($openshift kube list pods | grep registryPod)" in
    *Running*) echo -e "\n" && break;;
    *) echo -n "."
  esac
  sleep 0.5
done

echo -n "Waiting for Docker registry to become responsive.."
while true; do 
  curl http://localhost:5001 &>/dev/null && echo -e "\n" && break
  echo -n "."
  sleep 0.5
done

# show build cfgs
echo "Initially no build configurations:"
$openshift kube list buildConfigs

# define a build cfg
echo "Defining a new build configuration:"
$openshift kube create buildConfigs -c buildcfg/buildcfg.json

# show build cfgs
echo "Build configuration defined:"
$openshift kube list buildConfigs

# show no build running
echo "Initially no builds running:"
$openshift kube list builds

# request new build
echo "Triggering new build..."
curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @buildinvoke/pushevent.json http://localhost:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

# show build running
echo "Build now running:"
$openshift kube list builds

id=$($openshift kube list builds | grep new | awk '{print $1}')

echo -n "Waiting for build to complete.."
while true; do
  case "$($openshift kube get builds/$id)" in
    *complete*) echo -e "\n" && break;;
    *failed*)
      echo -e "failed\n\nBuild logs:\n"
      $openshift kube buildLogs --id="$id"
      exit 1
      ;;
    *) echo -n "."
  esac
  sleep 0.5
done

# Convert template to config
echo "Processing application template and applying the result config..."
$openshift kube process -c template/template.json | $openshift kube apply -c -

echo "Waiting for frontend service to start.."
while true
  out="$($openshift kube list services)"
  case "$out" in
    *frontend*) echo -e "\n" && break;;
    *) echo -n "."
  esac
done

echo -n "Waiting for frontend pod to start.."
while true; do 
  case "$($openshift kube list pods)" in
    *Running*) echo -e "\n" && break;;
    *) echo -n "."
  esac
  sleep 0.5
done

echo -n "Waiting on frontend to become responsive.."
while true; do 
  curl http://localhost:5432 &>/dev/null && echo -e "\n" && break
  echo -n "."
  sleep 0.5
done

echo "Curl frontend:"
curl http://localhost:5432