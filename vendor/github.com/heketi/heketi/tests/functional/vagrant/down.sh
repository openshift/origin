#!/bin/sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
cd "${SCRIPT_DIR}"

vagrant destroy -f
for i in $(virsh vol-list default | grep '\.disk' | awk '{print $1}'); do
    virsh vol-delete --pool default "${i}"
done
