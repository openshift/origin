#!/bin/bash

set -e

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

pushd ${hackdir}/../assets > /dev/null
  bundle exec grunt serve
popd > /dev/null