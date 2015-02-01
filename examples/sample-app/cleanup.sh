#!/bin/sh

echo "Killing openshift all-in-one server"
killall openshift
echo "Cleaning up openshift etcd content"
rm -rf openshift.local.etcd
echo "Cleaning up openshift etcd volumes"
rm -rf openshift.local.volumes
echo "Stopping all k8s docker containers on host"
docker ps | awk '{ print $NF " " $1 }' | grep ^k8s_ | awk '{print $2}' |  xargs -l -r docker stop
