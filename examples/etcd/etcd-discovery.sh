#!/bin/bash -e
#
# This is a wrapper for the etcd that serves as 'discovery' server and manager
# for the cluster configuration

address=$(getent ahosts ${HOSTNAME} | grep RAW | cut -d ' ' -f 1)

exec /usr/local/bin/etcd \
  --advertise-client-urls http://${address}:2379 \
  --listen-client-urls http://${address}:2379 \
  --data-dir /var/lib/etcd \
  --name discovery
