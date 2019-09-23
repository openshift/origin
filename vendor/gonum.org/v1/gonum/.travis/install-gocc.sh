#!/bin/bash

go get github.com/goccmack/gocc
cd $GOPATH/src/github.com/goccmack/gocc
git checkout $1
go install
