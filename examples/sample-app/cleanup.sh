#!/bin/sh

echo "Killing openshift all-in-one server"
killall openshift
echo "Cleaning up openshift etcd content"
rm -rf openshift.local.etcd
echo "Cleaning up openshift etcd volumes"
rm -rf openshift.local.volumes
echo "Killing all docker containers on host"
docker kill `docker ps --no-trunc -q`

