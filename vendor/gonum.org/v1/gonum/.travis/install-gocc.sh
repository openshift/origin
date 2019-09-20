#!/bin/bash

set -ex

# TODO(kortschak): Replace this with versioned go get when
# all our supported versions of Go support it.
mkdir -p $GOPATH/src/github.com/goccmack
git clone https://github.com/goccmack/gocc.git $GOPATH/src/github.com/goccmack/gocc
cd $GOPATH/src/github.com/goccmack/gocc
git checkout $1
go install
