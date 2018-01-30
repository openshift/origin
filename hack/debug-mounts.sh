#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::util::environment::setup_all_server_vars
os::util::ensure_tmpfs "${ETCD_DATA_DIR}"