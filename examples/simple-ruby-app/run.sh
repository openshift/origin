#!/bin/bash

# openshift server host
OSHOST=localhost

# openshift binary
openshift="../../_output/go/bin/openshift"

# OPTIONAL: Wipe out previous openshift/k8s deployment information for a 
# clean start.
rm -rf openshift.local.etcd

# OPTIONAL: kill all docker containers before starting
# docker kill `docker ps --no-trunc -q`


# Start the openshift all-in-one server
# (starts a kubernetes master and minion as well as providing the
# origin REST api)
# Uses Host docker socket so that the resulting docker images are 
# available on the host.  Otherwise the docker image lives only inside
# the docker build container, and goes away when the build container
# goes away.  (Currently the resulting image from the build is not
# pushed to any registry).
echo "Launching openshift all-in-one server"
USE_HOST_DOCKER_SOCKET=true $openshift start --listenAddr="0.0.0.0:8080" &> logs/openshift.log &

sleep 5

# Convert template to config
echo "Submitting template json for processing..."
curl -sld @template/template.json http://$OSHOST:8080/osapi/v1beta1/templateConfigs > processed/template.processed.json

# Deploy the config
$openshift kube -h http://$OSHOST:8080 apply -c processed/template.processed.json

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
sleep 5 

# Confirm the app is running/responsive.
echo "Frontend is available, sending request.  Frontend says:"
curl localhost:5432

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
curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @buildinvoke/pushevent.json http://$OSHOST:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

# webhook url
#http://$OSHOST:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github
#cd ~/demofiles/app
#edit app.rb
#git commit -am . ; git push origin master

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
  
echo "Your new application image is origin_ruby_sample: "
docker images | grep origin-ruby-sample
