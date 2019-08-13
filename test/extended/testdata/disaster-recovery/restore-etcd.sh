#!/bin/bash
set -euo pipefail

if [ -z "${BASTION_HOST}" ]; then exit 1; fi
if [ -z "${MASTERHOSTS}" ]; then exit 1; fi
if [ -z "${KUBE_SSH_KEY_PATH}" ]; then exit 1; fi

MASTERS=(${MASTERHOSTS})
FIRST_MASTER="${MASTERS[0]}"

function retry() {
  local ATTEMPTS="${1}"
  local rc=0
  shift
  for i in $(seq 0 $((ATTEMPTS-1))); do
    echo "--> ${@}"
    set +e
    "${@}"
    rc="$?"
    set -e
    echo "--> exit code: $rc"
    test "${rc}" = 0 && break
    sleep 10
  done
  return "${rc}"
}

function bastion_ssh() {
  retry 60 \
    ssh -o LogLevel=error -o ConnectionAttempts=100 -o ConnectTimeout=30 -o StrictHostKeyChecking=no \
        -o ProxyCommand="ssh -A -o StrictHostKeyChecking=no -o LogLevel=error -o ServerAliveInterval=30 -o ConnectionAttempts=100 -o ConnectTimeout=30 -W %h:%p core@${BASTION_HOST} 2>/dev/null" \
        $@
}

echo "Distribute snapshot across all masters"
for master in "${MASTERS[@]}"
do
  scp -o StrictHostKeyChecking=no -o ProxyCommand="ssh -A -o StrictHostKeyChecking=no -o ServerAliveInterval=30 -W %h:%p core@${BASTION_HOST}" ${KUBE_SSH_KEY_PATH} "core@${master}":/home/core/.ssh/id_rsa
  bastion_ssh "core@${master}" "sudo -i chmod 0600 /home/core/.ssh/id_rsa"
  bastion_ssh "core@${FIRST_MASTER}" "scp -o StrictHostKeyChecking=no /tmp/snapshot.db core@${master}:/tmp/snapshot.db"
done

echo "Collect etcd names"
for master in "${MASTERS[@]}"
do
  bastion_ssh "core@${master}" 'echo "etcd-member-$(hostname -f)" > /tmp/etcd_name && source /run/etcd/environment && echo "https://${ETCD_DNS_NAME}:2380" > /tmp/etcd_uri'
  bastion_ssh "core@${FIRST_MASTER}" "mkdir -p /tmp/etcd/${master} && scp -o StrictHostKeyChecking=no core@${master}:/tmp/etcd_name /tmp/etcd/${master}/etcd_name && scp -o StrictHostKeyChecking=no core@${master}:/tmp/etcd_uri /tmp/etcd/${master}/etcd_uri"
  bastion_ssh "core@${FIRST_MASTER}" "cat /tmp/etcd/${master}/etcd_name"
  bastion_ssh "core@${FIRST_MASTER}" "cat /tmp/etcd/${master}/etcd_uri"
done

echo "Assemble etcd connection string"
bastion_ssh "core@${FIRST_MASTER}" 'rm -rf /tmp/etcd/connstring && mapfile -t MASTERS < <(ls /tmp/etcd) && echo ${MASTERS[@]} && for master in "${MASTERS[@]}"; do echo -n "$(cat /tmp/etcd/${master}/etcd_name)=$(cat /tmp/etcd/${master}/etcd_uri)," >> /tmp/etcd/connstring; done && sed -i '"'$ s/.$//'"' /tmp/etcd/connstring'

echo "Restore etcd cluster from snapshot"
for master in "${MASTERS[@]}"
do
  echo "Running /usr/local/bin/etcd-snapshot-restore.sh on ${master}"
  bastion_ssh "core@${FIRST_MASTER}" "scp -o StrictHostKeyChecking=no /tmp/etcd/connstring core@${master}:/tmp/etcd_connstring"
  bastion_ssh "core@${master}" 'sudo -i /bin/bash -x /usr/local/bin/etcd-snapshot-restore.sh /tmp/snapshot.db $(cat /tmp/etcd_connstring)'
done
