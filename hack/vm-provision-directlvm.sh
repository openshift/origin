#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

if [[ $(grep openshift /home/vagrant/.bash_profile) = "" ]]; then
	echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/go/bin:\$GOPATH/bin:\$PATH" >> /home/vagrant/.bash_profile
  echo "cd \$GOPATH/src/github.com/openshift/origin" >> /home/vagrant/.bash_profile

  echo "bind '\"\e[A\":history-search-backward'" >> /home/vagrant/.bashrc
  echo "bind '\"\e[B\":history-search-forward'" >> /home/vagrant/.bashrc
else
  echo "vagrant bash shell already configured"
fi

if [[ $(grep openshift /root/.bash_profile) = "" ]]; then
  echo "export GOPATH=/home/vagrant/go" >> /root/.bash_profile
  echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/go/bin:\$GOPATH/bin:\$PATH" >> /root/.bash_profile
  echo "cd \$GOPATH/src/github.com/openshift/origin" >> /root/.bash_profile

  echo "bind '\"\e[A\":history-search-backward'" >> /root/.bashrc
  echo "bind '\"\e[B\":history-search-forward'" >> /root/.bashrc
else
  echo "root bash shell already configured"
fi

echo To install etcd, run hack/install-etcd.sh
