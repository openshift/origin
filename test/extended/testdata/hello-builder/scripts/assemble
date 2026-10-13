#!/bin/sh
unset GOPATH
cd /tmp/src
CGO_ENABLED=0 go build -a -installsuffix cgo -o hello-openshift -tags netgo
mv hello-openshift /tmp
