#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)

echo $(go version)

go get github.com/tools/godep golang.org/x/lint/golint

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
