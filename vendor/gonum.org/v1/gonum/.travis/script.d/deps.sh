#!/bin/bash

set -ex

# Avoid contaminating the go.mod/go.sum files.
# TODO(kortschak): Remove when golang/go#30515 is resolved
WORK=$(mktemp -d)
pushd $WORK

# Required for format check.
go get golang.org/x/tools/cmd/goimports
# Required for imports check.
go get gonum.org/v1/tools/cmd/check-imports
# Required for copyright header check.
go get gonum.org/v1/tools/cmd/check-copyright
# Required for coverage.
go get golang.org/x/tools/cmd/cover
go get github.com/mattn/goveralls
# Required for dot parser checks.
go get github.com/goccmack/gocc@66c61e9

# Clean up.
# TODO(kortschak): Remove when golang/go#30515 is resolved.
popd
rm -rf $WORK
