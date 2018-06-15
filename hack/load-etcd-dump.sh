#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

export BASETMPDIR="/tmp/openshift/load-etcd-dump"
rm -rf ${BASETMPDIR} || true

go run tools/testdebug/load_etcd_dump.go $@
