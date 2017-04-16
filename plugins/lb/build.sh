#!/bin/bash
set -x
pushd `dirname $0`/../..
hack/build-go.sh plugins/lb
cp _output/go/bin/lb plugins/lb/bin/
