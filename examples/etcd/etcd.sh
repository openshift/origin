#!/bin/bash
#
# This is a wrapper script for the etcd command.
# This wrapper detects the presence of ETCD_DISCOVERY environment variable and
# if this variable is set then it will use DNS lookup to collect the IP
# addresses of the other members of the cluster. This wrapper then adjust the
# size of the cluster in the discovery service and register itself.

# If we are not running in cluster, then just execute the etcd binary
if [[ -z "${ETCD_DISCOVERY-}" ]]; then
  exec /usr/local/bin/etcd "$@"
fi

address=$(getent ahosts ${HOSTNAME} | grep RAW | cut -d ' ' -f 1)

curl -sX PUT ${ETCD_DISCOVERY}/_config/size -d value=${ETCD_NUM_MEMBERS}

# Adding UNIX timestamp prevents having duplicate member id's
member_id="${HOSTNAME}-$(date +"%s")"

echo "Starting member ${member_id} (${address})..."
exec /usr/local/bin/etcd \
  -initial-advertise-peer-urls http://${address}:2380 \
  -listen-peer-urls http://${address}:2380 \
  -advertise-client-urls http://${address}:2379 \
  -listen-client-urls http://${address}:2379 \
  -data-dir /var/lib/etcd \
  -name ${member_id}
