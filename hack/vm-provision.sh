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
	echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/go/bin:\$GOPATH/bin:\$PATH" >> /home/vagrant/.bash_profile
  echo "cd \$GOPATH/src/github.com/openshift/origin" >> /home/vagrant/.bash_profile

  echo "bind '\"\e[A\":history-search-backward'" >> /home/vagrant/.bashrc
  echo "bind '\"\e[B\":history-search-forward'" >> /home/vagrant/.bashrc
fi

if [[ $(grep GOPATH /root/.bash_profile) = "" ]]; then
  touch /root/.bash_profile

  echo "export GOPATH=/home/vagrant/go" >> /root/.bash_profile
  echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/go/bin:\$GOPATH/bin:\$PATH" >> /root/.bash_profile
  echo "cd \$GOPATH/src/github.com/openshift/origin" >> /root/.bash_profile

  echo "bind '\"\e[A\":history-search-backward'" >> /root/.bashrc
  echo "bind '\"\e[B\":history-search-forward'" >> /root/.bashrc
else
  echo "root user path variables already configured"
fi

systemctl enable docker
systemctl start docker

usermod -a -G docker vagrant

echo To install etcd, run hack/install-etcd.sh

sed -i s/Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers
