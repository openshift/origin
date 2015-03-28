#!/bin/bash

readonly FAILOVER_IMAGE="openshift/origin-keepalived-failover-monitor"


function test_script() {
  echo "
export OPENSHIFT_ROUTER_NAME='roto-r00ter'
export OPENSHIFT_ROUTER_HA_VIRTUAL_IPS='10.0.2.100-102'
export OPENSHIFT_ROUTER_HA_NETWORK_INTERFACE='enp0s3'
# export OPENSHIFT_ROUTER_HA_UNICAST_PEERS='127.0.0.1'
# export OPENSHIFT_ROUTER_HA_USE_UNICAST='true'
export OPENSHIFT_ROUTER_HA_REPLICA_COUNT=1

/var/lib/openshift/keepalived/failover-monitor/monitor.sh
"

}


function start_failover_container() {
  echo $(test_script) |
    docker run -it --net=host --privileged=true --entrypoint=/bin/bash  \
               -v /lib/modules:/lib/modules $FAILOVER_IMAGE &

}

function run_image_verification_test() {
  local cname=$(start_failover_container)
  echo "  - started docker container $cname ..."

  #  Wait a bit for all the services to startup.
  sleep 60

  #  Dump logs and kill the container.
  echo "  - logs from container $cname:"
  docker logs $cname
  docker rm -f $cname
}


#
#  main():
#
run_image_verification_test

