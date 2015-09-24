#!/bin/bash
set -euo

yum update -y
yum install -y docker-io
systemctl enable docker
systemctl start docker

# Docker-in-docker is not compatible with SELinux enforcement
setenforce 0 || true

function set_env {
  USER_DIR="${1}"
  TARGET="${USER_DIR}/.bash_profile"
  if [[ $(grep _output ${TARGET}) = "" ]]; then
    touch "${USER_DIR}/.bash_profile"
    echo "export PATH=/vagrant/_output/local/go/bin:\$PATH" >> ${TARGET}
    echo "cd /vagrant" >> ${TARGET}
  else
    echo "path variables for ${USER_DIR} already configured"
  fi
}

set_env /home/vagrant
set_env /root
