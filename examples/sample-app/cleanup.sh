#!/bin/sh

echo "Killing openshift all-in-one server ..."
sudo pkill -x openshift

echo "Stopping all k8s docker containers on host ..."
sudo docker ps --format='{{.Names}}' | grep -E '^k8s_' | xargs -l -r sudo docker stop

echo "Unmounting openshift local volumes ..."
mount | grep "openshift.local.volumes" | awk '{ print $3}' | xargs -l -r sudo umount

echo "Cleaning up openshift runtime files ..."
sudo rm -rf openshift.local.*


