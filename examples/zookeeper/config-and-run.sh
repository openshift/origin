#!/bin/bash

# Persists the ID of the current instance of Zookeeper
echo ${SERVER_ID} > /opt/zookeeper/data/myid

# Substitute vars in configuration file
envsubst < /opt/zookeeper/conf/zoo-template.cfg > /opt/zookeeper/conf/zoo.cfg

exec /opt/zookeeper/bin/zkServer.sh start-foreground
