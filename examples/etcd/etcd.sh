#!/bin/bash
#
# This is a wrapper script for the etcd command.
# This wrapper detects the presence of ETCD_DISCOVERY environment variable and
# if this variable is set then it will use DNS lookup to collect the IP
# addresses of the other members of the cluster. This wrapper then adjust the
# size of the cluster in the discovery service and register itself.

# If we are not running in cluster, then just execute the etcd binary
if [[ -z "${ETCD_DISCOVERY_TOKEN-}" ]]; then
  exec /usr/local/bin/etcd "$@"
fi

# This variable is used by etcd server
export ETCD_DISCOVERY="${ETCD_DISCOVERY_URL}/v2/keys/discovery/${ETCD_DISCOVERY_TOKEN}"

# Set the size of this cluster to pre-defined number
# Will retry several times till the etcd-discovery service is not ready
for i in {1..5}; do
  echo "Attempt #${i} to update the cluster size in ${ETCD_DISCOVERY_URL} ..."
  etcdctl --peers "${ETCD_DISCOVERY_URL}" set discovery/${ETCD_DISCOVERY_TOKEN}/_config/size ${ETCD_NUM_MEMBERS} && break || sleep 2
done

# The IP address of this container
address=$(getent ahosts ${HOSTNAME} | grep RAW | cut -d ' ' -f 1)

# In case of failure when this container will be restarted, we have to remove
# this member from the list of members in discovery service. The new container
# will be added automatically and the data will be replicated.
ETCDCTL_PEERS="${ETCD_DISCOVERY_URL}"
initial_cluster=""
new_member=0

for member_url in $(etcdctl ls discovery/${ETCD_DISCOVERY_TOKEN}/); do
  out=$(etcdctl get ${member_url})
  if ! echo $out | grep -q "${address}"; then
    initial_cluster+="${out},"
    continue
  fi
  etcdctl rm ${member_url}
  member_id=$(echo "${member_url}" | cut -d '/' -f 4)
  new_member=1
  etcdctl --peers http://etcd:2379 member remove ${member_id}
  echo "Waiting for ${member_id} removal to propagate ..."
  sleep 3
done

# If this member already exists in the cluster, perform recovery using
# 'existing' cluster state.
if [ $new_member != 0 ]; then
  out=$(etcdctl --peers http://etcd:2379 member add ${HOSTNAME} http://${address}:2380 | grep ETCD_INITIAL_CLUSTER)
  echo "Waiting for ${HOSTNAME} to be added into cluster ..." && sleep 5
  eval "export ${out}"
  export ETCD_INITIAL_CLUSTER_STATE="existing"
  unset ETCD_DISCOVERY
fi

echo "Starting etcd member ${HOSTNAME} on ${address} ..."
exec /usr/local/bin/etcd \
  --initial-advertise-peer-urls http://${address}:2380 \
  --listen-peer-urls http://${address}:2380 \
  --advertise-client-urls http://${address}:2379 \
  --listen-client-urls http://127.0.0.1:2379,http://${address}:2379 \
  --data-dir /var/lib/etcd \
  --name ${HOSTNAME}
