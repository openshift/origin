#!/bin/bash

oc delete service zookeeper zookeeper-1 zookeeper-2 zookeeper-3
oc delete pod zookeeper-1 zookeeper-2 zookeeper-3
oc delete imageStream zookeeper-346-jdk7
