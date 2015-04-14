Native container routing using network infrastructure
==========================================================

Introduction
----------------------------------------------------------
This document describes how one could setup container networking using existing switches/routers and using
the kernel networking stack in Linux. The setup requires that the network administrator or some script
modifies the router[s] as new nodes are added to the cluster. The document will describe the steps to
use a Linux server as a simple router. The steps could then be adapted to any particular router.


Network Layout
----------------------------------------------------------
The diagram below shows the setup used in the document. It has one Linux node with two network interface
cards serving as a router, two switches and three nodes connected to these switches. 


Network Overview
----------------------------------------------------------
* 11.11.0.0/16 is the container network.
* 11.11.x.0/24 subnet is reserved for each node and assigned to the docker linux bridge.
* Each node has a route to the router for reaching anything 11.11.0.0/16 except the local subnet.
* Router has routes for each node, so it can direct to the right node.
* Nodes don't need any changes when new nodes are added unless the network topology is modified.
* IP forwarding is enabled on each node.
