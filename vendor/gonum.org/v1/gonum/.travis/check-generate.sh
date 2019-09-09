#!/bin/bash

go generate gonum.org/v1/gonum/blas
go generate gonum.org/v1/gonum/blas/gonum
go generate gonum.org/v1/gonum/unit
go generate gonum.org/v1/gonum/unit/constant
go generate gonum.org/v1/gonum/graph/formats/dot

# Discard changes to go.mod that have been made.
# FIXME(kortschak): Sort out a policy of what we should do with go.mod changes.
# If we want to check changes we should set `GOFLAGS=-mod=readonly` in .travis.yml.
git checkout -- go.{mod,sum}

if [ -n "$(git diff)" ]; then	
	git diff
	exit 1
fi
