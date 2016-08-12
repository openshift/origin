#!/bin/bash

# This script sets up a go workspace locally and invokes pythia to introspect code
# see: https://github.com/fzipp/pythia

# Prereq:
# go get github.com/fzipp/pythia
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# Check for `go` binary and set ${GOPATH}.
os::build::setup_env

pythia github.com/openshift/origin/cmd/openshift
