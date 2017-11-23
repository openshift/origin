#!/bin/bash
set -ex

go generate github.com/gonum/lapack/cgo/lapacke
if [ -n "$(git diff)" ]; then
	exit 1
fi
