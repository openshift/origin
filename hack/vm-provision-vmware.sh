#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

yum update -y
yum install -y docker-io git vim golang e2fsprogs tmux httpie ctags hg

if [[ ! -d /home/vagrant/go/src/github.com/openshift/origin ]]; then
  mkdir -p /home/vagrant/go/src/github.com/openshift/origin
  chown vagrant:vagrant /home/vagrant/go/src/github.com/openshift/origin
fi

if [[ $(grep GOPATH /home/vagrant/.bash_profile) = "" ]]; then
  touch /home/vagrant/.bash_profile
  echo "export GOPATH=/home/vagrant/go" >> /home/vagrant/.bash_profile
  echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/local/go/bin:\$GOPATH/bin:\$PATH" >> /home/vagrant/.bash_profile
  echo "cd \$GOPATH/src/github.com/openshift/origin" >> /home/vagrant/.bash_profile
fi

if [[ $(grep GOPATH /root/.bash_profile) = "" ]]; then
  touch /root/.bash_profile

  echo "export GOPATH=/home/vagrant/go" >> /root/.bash_profile
  echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/local/go/bin:\$GOPATH/bin:\$PATH" >> /root/.bash_profile
  echo "cd \$GOPATH/src/github.com/openshift/origin" >> /root/.bash_profile

else
  echo "root user path variables already configured"
fi

systemctl enable docker
systemctl start docker

usermod -a -G docker vagrant

echo To install etcd, run hack/install-etcd.sh

sed -i s/Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers