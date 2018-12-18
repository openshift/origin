#!/bin/bash

go generate gonum.org/v1/gonum/blas
go generate gonum.org/v1/gonum/blas/gonum
go generate gonum.org/v1/gonum/unit
go generate gonum.org/v1/gonum/graph/formats/dot
if [ -n "$(git diff)" ]; then
	git diff
	exit 1
fi
