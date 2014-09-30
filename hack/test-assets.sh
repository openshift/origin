#!/bin/bash

set -e

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS}" == "true" && "${TEST_ASSETS}" == "false" ]]; then
  exit
fi

pushd assets > /dev/null
  grunt test
  grunt build
popd > /dev/null

Godeps/_workspace/bin/go-bindata -prefix "assets/dist" -pkg "assets" -o "test/assets/bindata.go" assets/dist/...

echo "Validating checked in bindata.go is up to date..."
# TODO remove the pipe to head as it messes up the exit code
diff test/assets/bindata.go pkg/assets/bindata.go | head -n 100