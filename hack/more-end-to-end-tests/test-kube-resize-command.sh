#!/bin/bash

# Tests the openshift kube resize command
# This scripts needs to be called and sourced by the test-end-to-end.sh script to initialize the correct
# required variables

# Wait for replication controller to spawn pods and then wait for upscale and then downscale
# $1 namespace
function wait_for_replication_controller_resize() {
    namespace=$1
    
    REPLICATION_CONTROLLER_CONFIG_FILE=examples/hello-openshift/hello-replication-controller.json

    echo "[INFO] Creating project $1 "
    openshift ex new-project $namespace

    echo "[INFO] Waiting for replication controller to start in $1 namespace"
    osc create -f ${REPLICATION_CONTROLLER_CONFIG_FILE} -n $namespace
    replicas=3
    replication_controller_name=replication-controller-test-1

    sleep 10
    # the replication-controller-test spawns three replicas
    wait_for_command "osc get pods -n $namespace | grep Running | grep -i ${replication_controller_name} | wc -l | grep -x $replicas" $((120*TIME_SEC))
    echo "[INFO] Replication controller ${replication_controller_name} spwanned the number or requested pods : ${replicas}"


    replicas=2
    openshift kube resize --replicas=$replicas rc ${replication_controller_name} -n $namespace
    wait_for_command "osc get pods -n $namespace | grep Running | grep -i ${replication_controller_name} | wc -l | grep -x $replicas" $((120*TIME_SEC))
    echo "[INFO] Replication controller ${replication_controller_name} spwanned the number or requested pods : ${replicas}"
 
    replicas=4
    openshift kube resize --replicas=$replicas rc ${replication_controller_name} -n $namespace
    wait_for_command "osc get pods -n $namespace | grep Running | grep -i ${replication_controller_name} | wc -l | grep -x $replicas" $((120*TIME_SEC))
    echo "[INFO] Replication controller ${replication_controller_name} spwanned the number or requested pods : ${replicas}"
 

    replicas=0
    # Because of set +e on wait_for_command, doing a grep on counts will fail, so we have to check that pods are destroyed in another manner
    openshift kube resize --replicas=$replicas rc ${replication_controller_name} -n $namespace
    wait_for_command "osc get pods -n $namespace | wc -l | grep -x 1" $((120*TIME_SEC))
    echo "[INFO] Replication controller ${replication_controller_name} spwanned the number or requested pods : ${replicas}"

}


# Start here the different methods

wait_for_replication_controller_resize "testrc"
