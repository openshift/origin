#!/bin/bash

# openshift server host
OSHOST=localhost

# openshift binary
openshift="../../_output/go/bin/openshift"

# Wipe out previous openshift/k8s deployment information for a clean start.
rm -rf openshift.local.etcd

# Start the openshift all-in-one server and sets docker registry that will
# run on k8s itself.
mkdir -p /tmp/docker
INTERFACE=${1:-"docker0"}
REGISTRY=$(ifconfig $INTERFACE 2> /dev/null | awk '/inet addr:/ {print $2}' | sed 's/addr://')
echo "Launching openshift all-in-one server with following DOCKER_REGISTRY: "$REGISTRY":5000"
DOCKER_REGISTRY=$REGISTRY":5000" $openshift start --listenAddr="0.0.0.0:8080" &> openshift.log &

sleep 5

# Deploy the docker config
$openshift kube -h http://$OSHOST:8080 apply -c registry/docker-registry.json

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

# Sometimes the app isn't quite available even though the pod is running, wait a little longer.
sleep 5

# Confirm the app is running/responsive.
echo "Registry is available, sending request.  Registry says:"
curl localhost:5000
echo ""
go run registry/list.go

# Define a build cfg
echo "Defining a new build configuration"
$openshift kube create buildConfigs -c build/buildcfg.json

# Request new build
echo "Triggering new build"
curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @build/pushevent.json http://$OSHOST:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

# Show build running
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

# List registry contents
echo "Registry contents are: "
go run registry/list.go

