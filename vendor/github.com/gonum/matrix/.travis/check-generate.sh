#!/bin/bash

go generate github.com/gonum/matrix
if [ -n "$(git diff)" ]; then
	exit 1
fi
