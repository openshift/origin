#!/bin/bash

set -e

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

pushd ${hackdir}/../assets > /dev/null
  rm -f debug.zip
  zip -r debug .tmp/ dist/
popd > /dev/null

