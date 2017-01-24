#!/bin/bash
#
# https://bugzilla.redhat.com/show_bug.cgi?id=1415570
#
# In the initial release of OCP 3.4, paths for two objects,
# User and EgressNetworkPolicy, inadvertantly changed.
# This script migrates any of these resources created in
# version of OCP 3.4 without the fix to the proper location
# in etcd. Namely:
#
# identities -> useridentities
# egressnetworkpolicies -> registry/egressnetworkpolicy

USAGE="${0} [-a] [-c os-master-config-dir] [-p os-etcd-prefix] etcd-endpoints"
usage() {
  echo "${USAGE}"
  exit 1
}

APPLY=false
OS_MASTER_CONFIG_DIR="/etc/origin/master"
OS_ETCD_PREFIX="/openshift.io"

while getopts ":ac:p:" opt; do
  case $opt in
    a)
      APPLY=true
      ;;
    c)
      OS_MASTER_CONFIG_DIR="${OPTARG}"
      ;;
    p)
      OS_ETCD_PREFIX="${OPTARG}"
      ;;
    \?)
      usage
      ;;
    :)
      echo "Option -$OPTARG requires an argument"
      usage
      ;;
  esac
done
shift $((OPTIND-1))

export ETCDCTL_ENDPOINT=${1:-""}
export ETCDCTL_CA_FILE=${ETCDCTL_CA_FILE:-"${OS_MASTER_CONFIG_DIR}/master.etcd-ca.crt"}
export ETCDCTL_CERT_FILE=${ETCDCTL_CERT_FILE:-"${OS_MASTER_CONFIG_DIR}/master.etcd-client.crt"}
export ETCDCTL_KEY_FILE=${ETCDCTL_KEY_FILE:-"${OS_MASTER_CONFIG_DIR}/master.etcd-client.key"}

if [[ ! -e "${ETCDCTL_CA_FILE}" ]]; then
  ETCDCTL_CA_FILE="${OS_MASTER_CONFIG_DIR}/ca.crt"
  if [[ ! -e "${ETCDCTL_CA_FILE}" ]]; then
    echo "Default CA files not found. Please specify correct ETCDCTL_CA_FILE."
    exit 1
  fi
fi

if [[ ! -e "${ETCDCTL_CERT_FILE}" ]]; then
  echo "Default client cert file not found. Please specify correct ETCDCTL_CERT_FILE."
  exit 1
fi

if [[ ! -e "${ETCDCTL_KEY_FILE}" ]]; then
  echo "Default client key file not found. Please specify correct ETCDCTL_KEY_FILE."
  exit 1
fi

if [[ -z "${ETCDCTL_ENDPOINT}" ]]; then
  echo "etcd-endpoints required"
  usage
fi

if [[ "$APPLY" != "true" ]]; then
  echo "Running in dry-run mode. Use -a option to apply changes."
fi

if ! command -v etcdctl &>/dev/null; then
  echo "This utility requires etcdctl to be installed"
  exit 1
fi

echo_mode() {
  if [[ "$APPLY" != "true" ]]; then
    echo "dry-run:" "$@"
  else
    echo "$@"
  fi
}

copy_key() {
  echo_mode "copying ${1} to ${2}"
  if ! value="$(etcdctl get "${1}")"; then
    echo "failed to get key ${1}"
    return 1
  fi
  if etcdctl get "${2}" &>/dev/null; then
    echo_mode "overwriting existing key ${2}"
  fi
  if [[ "$APPLY" = "true" ]]; then
    if ! etcdctl set "${2}" "$value" >/dev/null; then
        echo "failed to set key ${2}"
        return 1
    fi
  fi
  return 0
}

copy_keys() {
  for key in $(etcdctl ls "${1}"); do
    newkey="${2}/$(basename "${key}")"
    copy_key "${key}" "${newkey}"
  done
}

echo "Migrating Users"
copy_keys "${OS_ETCD_PREFIX}/identities" "${OS_ETCD_PREFIX}/useridentities"

echo "Migrating Egress Policies"
for project in $(etcdctl ls "${OS_ETCD_PREFIX}/egressnetworkpolicies"); do
  projectname="$(basename "${project}")"
  copy_keys "${OS_ETCD_PREFIX}/egressnetworkpolicies/${projectname}" "${OS_ETCD_PREFIX}/registry/egressnetworkpolicy/${projectname}"
done 
