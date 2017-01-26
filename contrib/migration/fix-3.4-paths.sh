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

USAGE="${0} [-a] [-c os-master-config-dir] [-p os-etcd-prefix] [-b backup-dir] etcd-endpoints"
usage() {
  echo "${USAGE}"
  exit 1
}

# default values
APPLY=false
OS_MASTER_CONFIG_DIR="/etc/origin/master"
OS_ETCD_PREFIX="/openshift.io"
BACKUP_DIR="$HOME/openshift-3.4-migration-backup"

while getopts ":ac:p:b:" opt; do
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
    b)
      BACKUP_DIR="${OPTARG}"
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
else
  if ! mkdir -p "${BACKUP_DIR}"; then
    echo "Unable to create backup directory ${BACKUP_DIR}"
    exit 1
  fi
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

backup_key() {
  key="${1}"
  value="${2}"

  backupfile="${BACKUP_DIR}/${key}"
  mkdir -p "$(dirname "${backupfile}")"
  echo "$value" > "${backupfile}"
}

copy_key() {
  echo_mode "copying ${1} to ${2}"
  if ! value="$(etcdctl get "${1}")"; then
    echo_mode "failed to get key ${1}"
    exit 1
  fi
  if existing=$(etcdctl get "${2}" 2>/dev/null); then
    echo_mode "overwriting existing key ${2}"
  fi
  if [[ "$APPLY" = "true" ]]; then
    backup_key "${1}" "${value}"
    if [[ -n "${existing}" ]]; then
      backup_key "${2}" "${existing}"
    fi
    if ! etcdctl set "${2}" "$value" >/dev/null; then
      echo "failed to set key ${2}"
      exit 1
    fi
    if ! etcdctl rm "${1}" >/dev/null; then
      echo "failed to remove old key ${1}"
      exit 1
    fi
  fi
  return 0
}

copy_keys() {
  output="$(etcdctl ls "${1}")"
  if [[ $? -ne 0 || -z "$output" ]]; then
    echo_mode "No keys found to migrate"
    return
  fi
  for key in $output; do
    newkey="${2}/$(basename "${key}")"
    copy_key "${key}" "${newkey}"
  done
}

IFS=$'\n'

echo_mode "Migrating Users"
copy_keys "${OS_ETCD_PREFIX}/identities" "${OS_ETCD_PREFIX}/useridentities"

echo_mode "Migrating Egress Policies"
output="$(etcdctl ls "${OS_ETCD_PREFIX}/egressnetworkpolicies")"
if [[ $? -ne 0 || -z "$output" ]]; then
  echo_mode "No keys found to migrate"
else
  for project in $output; do
    projectname="$(basename "${project}")"
    echo_mode "Project $projectname"
    copy_keys "${OS_ETCD_PREFIX}/egressnetworkpolicies/${projectname}" "${OS_ETCD_PREFIX}/registry/egressnetworkpolicy/${projectname}"
  done
fi
