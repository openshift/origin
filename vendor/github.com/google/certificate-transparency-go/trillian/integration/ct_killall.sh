#!/bin/bash
# Kill all ctfe/trillian related processes.
killall $@ ct_server
killall $@ trillian_log_server
killall $@ trillian_log_signer
if [[ -x "${ETCD_DIR}/etcd" ]]; then
  killall $@ etcd
  if [[ -x "${PROMETHEUS_DIR}/prometheus" ]]; then
    killall $@ etcdiscover
    killall $@ prometheus
  fi
fi
