#!/bin/sh
source /run/$NAME-env

exec /usr/bin/openshift start master --config=${CONFIG_FILE} $OPTIONS
