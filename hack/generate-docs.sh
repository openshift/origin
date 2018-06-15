#!/bin/bash

# This file is not intended to be run automatically. It is meant to be run
# immediately before exporting docs. We do not want to check these documents in
# by default.

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# Generate fresh docs
os::util::gen-docs ${1:-""}
