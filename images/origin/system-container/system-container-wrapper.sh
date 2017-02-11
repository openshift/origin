#!/bin/sh
source /run/$NAME-env

exec /usr/bin/openshift start master $COMMAND --config=${CONFIG_FILE} $OPTIONS
