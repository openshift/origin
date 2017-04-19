#!/bin/bash

#  Includes.
source "$(dirname "${BASH_SOURCE[0]}")/lib/failover-functions.sh"


#
#  main():
#
setup_failover

start_failover_services

echo "`basename $0`: OpenShift IP Failover service terminated."

