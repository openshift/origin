#!/usr/bin/env bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::log::info "Starting containerized end-to-end test"

# cluster up no longer produces a cluster that run the e2e test.  These use cases are already mostly covered
# in existing e2e suites.  The image-registry related tests stand out as ones that may not have an equivalent.
