#!/bin/bash

#  Includes.
source "$(dirname "${BASH_SOURCE[0]}")/lib/failover-functions.sh"


#
#  main():
#
setup_failover

start_failover_services

echo "In $0: waiting for router container to terminate ..."

tail -f /dev/null
# container_name=$(get_docker_container_name "$ROUTER_NAME")
# docker wait $container-name
