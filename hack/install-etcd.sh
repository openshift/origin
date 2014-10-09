#!/bin/bash

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

mkdir -p _output
cd _output
git clone https://github.com/coreos/etcd.git
cd etcd
git checkout $(go run ${hackdir}/version.go ${hackdir}/../Godeps/Godeps.json github.com/coreos/etcd/server)
./build

