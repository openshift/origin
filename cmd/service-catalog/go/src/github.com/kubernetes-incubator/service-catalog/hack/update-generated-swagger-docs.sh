#!/bin/bash

# Script to generate docs from the latest swagger spec.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

pushd "${OS_ROOT}/hack/swagger-doc" > /dev/null
gradle gendocs --info
popd > /dev/null

os::log::info "Swagger doc generation successful"
