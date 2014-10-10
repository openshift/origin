#!/bin/bash

if [[ `pgrep openshift` ]]
then
    echo "Killing openshift all-in-one server"
    kill -9  `pgrep openshift`
fi
echo "Cleaning up openshift etcd content"
rm -rf openshift.local.etcd
echo "Cleaning up openshift etcd volumes"
rm -rf openshift.local.volumes
echo "Killing all docker containers on host"
docker rm -f `docker ps --no-trunc -q`
