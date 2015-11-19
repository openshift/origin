#!/bin/bash
set -euo

USERNAME=vagrant
ORIGIN_ROOT=${1:-/vagrant}

yum update -y
yum install -y docker-io go git

# It should be safe to bypass security to access docker in a dev
# environment.  This also allows bash completion, which doesn't work
# when invoking a target command via sudo.
if ! getent group docker > /dev/null; then
  groupadd docker
fi
usermod -a -G docker "${USERNAME}"

systemctl enable docker
systemctl start docker

# Docker-in-docker is not compatible with SELinux enforcement
setenforce 0 || true

function set_env {
  USER_DIR="${1}"
  # Prefer bashrc to bash_profile since bash_profile is only loaded on
  # login and bashrc is loaded by bash_profile anyway.
  TARGET="${USER_DIR}/.bashrc"
  OUTPUT_PATH="${ORIGIN_ROOT}/_output/local"
  if [[ $(grep ${OUTPUT_PATH} ${TARGET}) = "" ]]; then
    echo "export GOPATH=${OUTPUT_PATH}/go" >> ${TARGET}
    # Binpath for origin binaries
    echo "export PATH=${OUTPUT_PATH}/bin/linux/amd64:\$PATH" >> ${TARGET}
    # Binpath for go-getted binaries (e.g. ginkgo)
    echo "export PATH=${OUTPUT_PATH}/go/bin:\$PATH" >> ${TARGET}
    echo "cd ${ORIGIN_ROOT}" >> ${TARGET}
  else
    echo "path variables for ${USER_DIR} already configured"
  fi
}

set_env "/home/${USERNAME}"
set_env /root

# Ensure ginkgo is available for running e2e tests
su - "${USERNAME}" -c 'go get github.com/onsi/ginkgo/ginkgo'
