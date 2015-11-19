#!/bin/sh

echo "Killing openshift all-in-one server ..."
pkill -x openshift

echo "Cleaning up openshift runtime files ..."
rm -rf openshift.local.*

echo "Stopping all k8s docker containers on host ..."
docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
