#!/bin/bash
set -ex

go generate github.com/gonum/blas/native
go generate github.com/gonum/blas/cgo
if [ -n "$(git diff)" ]; then
	exit 1
fi
