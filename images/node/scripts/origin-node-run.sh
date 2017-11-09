#!/bin/sh

set -eu

conf=${CONFIG_FILE:-/etc/origin/node/node-config.yaml}
if [[ "$#" -eq 0 ]]; then
  eval "set -- ${OPTIONS:---loglevel=2}"
fi

exec /usr/bin/openshift start node "--config=${conf}" "$@"
