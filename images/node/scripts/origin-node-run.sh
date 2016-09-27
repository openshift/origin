#!/bin/sh

set -eu

conf=${CONFIG_FILE:-/etc/origin/node/node-config.yaml}
opts=${OPTIONS:---loglevel=2}
if [ "$#" -ne 0 ]; then
  opts=""
fi

exec /usr/bin/openshift start node "--config=${conf}" "${opts}" $@
