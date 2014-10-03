#!/bin/bash

set -e

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS}" == "true" && "${TEST_ASSETS}" == "false" ]]; then
  exit
fi

pushd ${hackdir}/../assets > /dev/null
  bundle exec grunt test
  bundle exec grunt build
popd > /dev/null

pushd ${hackdir}/../ > /dev/null
  echo "Calculating asset checksums..."
  find assets/dist -type f | sort | xargs md5sum

  Godeps/_workspace/bin/go-bindata -prefix "assets/dist" -pkg "assets" -o "_output/test/assets/bindata.go" assets/dist/...

  echo "Validating checked in bindata.go is up to date..."
  # TODO remove the pipe to head as it messes up the exit code
  diff _output/test/assets/bindata.go pkg/assets/bindata.go
popd > /dev/null