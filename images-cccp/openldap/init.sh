#!/bin/bash
set -x

/usr/local/bin/test-init.sh &
exec /usr/local/bin/run-openldap.sh