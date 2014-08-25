#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

yum update -y
yum install -y docker-io git vim golang e2fsprogs tmux httpie ctags hg

if [[ $(grep GOPATH /home/vagrant/.bash_profile) = "" ]]; then
	touch /home/vagrant/.bash_profile
	mkdir /home/vagrant/go
	chown vagrant:vagrant /home/vagrant/go 
	echo "export GOPATH=/home/vagrant/go" >> /home/vagrant/.bash_profile
	echo "export PATH=\$GOPATH/bin:\$PATH" >> /home/vagrant/.bash_profile
fi

systemctl enable docker
systemctl start docker

usermod -a -G docker vagrant

echo "Get etcd with the following command:"
echo "    go get github.com/coreos/etcd"

sed -i s/Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers
