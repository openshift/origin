#!/usr/bin/env bash
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# Update TLS artifacts
go run -mod vendor ./cmd/update-tls-artifacts generate-ownership

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
