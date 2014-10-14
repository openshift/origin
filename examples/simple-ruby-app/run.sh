#!/bin/bash

# OpenShift binary
openshift="../../_output/go/bin/openshift"

# OPTIONAL: Wipe out previous OpenShift/k8s deployment information for a
# clean start.
rm -rf openshift.local.etcd

# OPTIONAL: kill all Docker containers before starting
# docker kill `docker ps --no-trunc -q`

echo "Pre-pulling images"
./pullimages.sh

# Start the OpenShift all-in-one server
# (starts a kubernetes master and minion as well as providing the origin REST api)
echo "Launching openshift all-in-one server"
$openshift start &> logs/openshift.log &

sleep 5

# Deploy the private Docker registry config
$openshift kube apply -c registry-config.json

# Wait for the app container to start up
rc=1
while [ ! $rc -eq 0 ]
do
  echo "Waiting for Docker registry pod to start..."
  $openshift kube list pods
  sleep 5
  $openshift kube list pods | grep registryPod | grep Running
  rc=$?
done

$openshift kube list services | grep frontend
rc=$?
while [ ! $rc -eq 0 ]
do
  echo "Waiting for Docker registry service to start..."
  $openshift kube list services
  sleep 5
  $openshift kube list services | grep registryPod
  rc=$?
done

# poke the docker registry to make sure it's alive
echo "Probing docker registry"
curl http://localhost:5001 >& /dev/null
curl http://localhost:5001

# show build cfgs
echo "Initially no build configurations:"
$openshift kube list buildConfigs

# define a build cfg
echo "Defining a new build configuration"
$openshift kube create buildConfigs -c buildcfg/buildcfg.json

# show build cfgs
echo "Build configuration defined:"
$openshift kube list buildConfigs


#show no build running
echo "Initially no builds running:"
$openshift kube list builds


#Requesting new build
echo "Triggering new build"
curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @buildinvoke/pushevent.json http://localhost:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

#show build running
echo "Build now running: "
$openshift kube list builds

id=`$openshift kube list builds | grep new | awk '{print $1}'`

$openshift kube get builds/$id | grep complete
rc=$?
while [ ! $rc -eq 0 ]
do
  echo "Waiting for build to complete..."
  $openshift kube get builds/$id
  sleep 5
  $openshift kube get builds/$id | grep complete
  rc=$?
done

# Convert template to config
echo "Submitting application template json for processing..."
$openshift kube process -c template/template.json | $openshift kube apply -c -

# Wait for the app container to start up
rc=1
while [ ! $rc -eq 0 ]
do
  echo "Waiting for frontend pod to start..."
  $openshift kube list pods
  sleep 5
  $openshift kube list pods | grep frontend | grep Running
  rc=$?
done

$openshift kube list services | grep frontend
rc=$?
while [ ! $rc -eq 0 ]
do
  echo "Waiting for frontend service to start..."
  $openshift kube list services
  sleep 5
  $openshift kube list services | grep frontend
  rc=$?
done

# Sometimes the app isn't quite available even though the pod is running,
# wait a little longer.
sleep 20

# Confirm the app is running/responsive.
echo "Frontend is available, sending request.  Frontend says:"
curl http://localhost:5432
