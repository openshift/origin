#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Enable overlayfs for dind if it can be tested to work.
function enable-overlay-storage() {
  local storage_dir=${1:-/var/lib/docker}

  local msg=""

  if grep -q overlay /proc/filesystems; then
    # Smoke test the overlay filesystem:

    # 1. create smoke dir in the storage dir being mounted
    local d="${storage_dir}/smoke"
    mkdir -p "${d}/upper" "${d}/lower" "${d}/work" "${d}/mount"

    # 2. try to mount an overlay fs on top of the smoke dir
    local overlay_works=1
    mount -t overlay overlay\
          -o"lowerdir=${d}/lower,upperdir=${d}/upper,workdir=${d}/work"\
          "${d}/mount" &&\
    # 3. try to write a file in the overlay mount
          echo foo > "${d}/mount/probe" || overlay_works=

    umount -f "${d}/mount" || true
    rm -rf "${d}" || true

    if [[ -n "${overlay_works}" ]]; then
      msg="Enabling overlay storage for docker-in-docker"
      sed -i -e 's+vfs+overlay+' /etc/sysconfig/docker-storage
    fi
  fi

  if [[ -z "${msg}" ]]; then
    msg="WARNING: Unable to enable overlay storage for docker-in-docker"
  fi

  echo "${msg}"
}

mount --make-shared /
mount --make-shared /run
enable-overlay-storage
