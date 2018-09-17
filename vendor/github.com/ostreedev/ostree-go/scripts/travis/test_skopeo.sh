#!/bin/bash
set -euo pipefail

go get github.com/LK4D4/vndr

mkdir -p $GOPATH/src/github.com/containers
cd $GOPATH/src/github.com/containers
git clone https://github.com/containers/skopeo
cd skopeo
sed -i -e 's|^github.com/ostreedev/ostree-go.*$|github.com/ostreedev/ostree-go HEAD /ostree-go|g' vendor.conf
$GOPATH/bin/vndr
make binary-local
mkdir -p /ostree/repo
ostree --repo=/ostree/repo init --mode=bare-user
mkdir -p /etc/containers/
cp -u default-policy.json /etc/containers/policy.json
./skopeo copy docker://docker.io/fedora ostree:docker.io/fedora@/ostree/repo
