#!/bin/bash -e

#  Constants.
readonly TEST_DIR=$(dirname "${BASH_SOURCE[0]}")
readonly FAILOVER_IMAGE="openshift/origin-keepalived-ipfailover"
readonly TEST_VIPS="10.0.2.100-102,2001:DB8:1ABC::1F39-1F3B"
readonly MONITOR_PORT="12345"


function stop_echo_server() {
  local pid=$1
  if [ -z "$pid" ]; then
    pid=$(ps -e -opid,args | grep echoserver.py | grep -v grep | awk '{print $1}')
  fi

  #  Send SIGUSR1 to the echo server to terminate it.
  [ -n "$pid" ] && kill -s USR1 $pid
}


function start_echo_server() {
  stop_echo_server

  export PORT=${MONITOR_PORT}
  nohup python ${TEST_DIR}/echoserver.py &> /dev/null &
  echo $!
}


function start_failover_container() {
  local cfg="-e OPENSHIFT_HA_CONFIG_NAME="roto-r00ter""
  local vips="-e OPENSHIFT_HA_VIRTUAL_IPS="${TEST_VIPS}""
  local netif="-e OPENSHIFT_HA_NETWORK_INTERFACE="enp0s3""
  local port="-e OPENSHIFT_HA_MONITOR_PORT="${MONITOR_PORT}""
  # local unicast="-e export OPENSHIFT_HA_USE_UNICAST="true""
  # local unicastpeers="-e OPENSHIFT_HA_UNICAST_PEERS="127.0.0.1""
  local selector="-e OPENSHIFT_HA_SELECTOR="""
  local envopts="$cfg $vips $netif $port $unicast $unicastpeers $selector"

  docker run -dit --net=host --privileged=true   \
         -v /lib/modules:/lib/modules $envopts $FAILOVER_IMAGE &

}


function run_image_verification_test() {
  echo "  - starting echo server ..."
  local pid=$(start_echo_server)
  echo "  - started echo server pid=$pid ..."

  #  On interrupt, cleanup - stop echo server.
  trap "stop_echo_server $pid" INT

  local cname=$(start_failover_container)
  echo "  - started docker container $cname ..."

  #  Wait a bit for all the services to startup.
  sleep 10

  #  Check container is up and has keepalived processes.
  local cmd="ps -ef  | grep '/usr/sbin/keepalived' | grep -v grep | wc -l"
  local numprocs=$(echo "$cmd" | docker exec -i $cname /bin/bash)

  #  Stop echo server.
  stop_echo_server $pid

  if [[ -n "$numprocs" && $numprocs -gt 0 ]]; then
    #  Success - print info and kill the container.
    echo "  - There are $numprocs keepalived processes running"
    echo "  - Cleaning up docker containers ..."
    docker rm -f $cname
    echo "  - All tests PASSED."
    return 0
  fi

  #  Failure - print info and dump logs (keep the docker container around
  #  for debugging).
  echo "  - There are $numprocs keepalived processes running"
  echo "  - logs from container $cname:"
  docker logs $cname || :
  echo "  - Test FAILED."
  exit 1
}


#
#  main():
#
run_image_verification_test

