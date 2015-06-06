# High Availability (HA) Configuration

## Problem
In the current OpenShift operational flow, one or more HAProxy routers are
used to direct network traffic to the target services. This makes use of
HAProxy as a layer-7 load balancer to reach multiple services/applications.
This require the HAProxy routers to be "discoverable" - meaning the IP
address resolution scheme needs to know where each HAProxy router is
running. This "discovery" mechanism in most cases would be via DNS but
that doesn't preclude using some other service discovery mechanism ala
via zookeeper or etcd.

In any case, that methodology works fine in steady state conditions.
It does, however have implications on failure conditions where the machine
running the router process goes down or the router process dies or there
is a network split. When these failure conditions arise, the above model
breaks down as it requires the caller (or something in the caller's
execution chain) to implement either a health checker or retry failed
requests and/or update the "discovery" mechanism to remove the failed
instance(s) from the traffic mix. Otherwise, a certain subset of requests
(failed/total) will fail via the upstream "discovery" mechanism (e.g. DNS).


## Use Cases
  1. As an administrator, I want my cluster to be assigned a resource set
     and I want the cluster to automatically manage those resources.
  2. As an administrator, I want my cluster to be assigned a set of virtual
     IP addresses that the cluster manages and migrates (with zero or
     minimal downtime) on failure conditions.
  3. As an addendum to use case #2, the administrator should not be
     required to perform any manual interaction to update the upstream
     "discovery" sources (e.g. DNS). The cluster should service all the
     assigned virtual IPs when atleast a single node is available - and
     this should be in spite of the fact that the current available
     resources are not sufficient to reach "critical mass" aka the
     desired state.


## Goals
The goal here is to provide the OpenShift environment with one or more
floating Virtual IP addresses which can be automatically migrated across
the cluster when the target resource (the HAProxy router specific to the
above mentioned problem) is not available.


## Basic Concepts
This proposal adds a new admin command that allows an administrator the
ability to setup a high availability configuration on a selection of nodes.

### Proposed Syntax (production):

        openshift admin ha-config [<name>] <options>

        where:
            <name> = Name of the HA configuration.
                     Default: generated name (e.g.  ha-config-1)
            <options> = One or more of:
                --type=keepalived  #  For now, always keepalived.
                --create
                --credentials=<credentials>
                --no-headers=<headers>
                -o|--output=<format>
                --output-version=<version>
                -t, --template=<template>
                --images=<image>
                --latest-images=<latest>
                -l,--selector=<selector>
                --virtual-ips=<ip-range>
                -i|--interface=<interface>
                -w|--watch-port=<port>
                -u|--unicast  # optional for now - add support later.
            <credentials> = <string> - Path to .kubeconfig file containing
                                       the credentials to use to contact
                                       the master.
            <headers> = true|false - When using default output, whether or
                                     not to print headers.
            <format> = Output format.
                       One of: json|yaml|template|templatefile
            <version> = <string> - Output the formatted object with the
                                   given version (default: api-version)
            <template> = <string> - Template string or path to the template
                                    file to use when -o=template or -o=templatefile.
                                    The template format is golang templates
                                    [http://golang.org/pkg/text/template/#pkg-overview]
            <image> = openshift/origin-<component>:<version> - Image to use.
            <component> = <type>-ha-config - image component name, default
                                             is based on the type.
                                             Default: keepalived-ha-config
            <latest> = true|false - If true, will attempt to use the latest
                                    image instead of the latest release
                                    for the HA sidecar component.
            <selector> = <string> - The node selector to use for running
                                    the HA sidecar pods.
            <ip-range> = string - One or more comma separated IP address
                                  or ranges.
                                  Example: 10.2.3.42,10.2.3.80-84,10.2.3.21
            <interface> = <string> - The interface to use.
                                     Default: Default interface on node or eth0
            <port> = <number> - Port to watch for resource availability.
                                Default: 80.
            <string> = a string of characters.
            <number> = a number ([0-9]*).


## Examples
Examples:

       $ # View the HA configuration.
       $ openshift admin ha-config -o yaml

       $ # Create HA configuration with IP failover serving 5 virtual IPS.
       $ openshift admin ha-config --virtual-ips="10.1.1.5-8,42.42.42.42" \
                                   --selector="jack=the-vipper"           \
                                   --create

       $ # Create a HA configuration with IP failover but disabling the
       $ # VRRP multicast group (using unicast instead).
       $ # Note: Initial release **will** likely not have unicast support.
       $ openshift admin ha-config ha-amzn --unicast=true               \
                                   --selector="ha-router=amzn-us-west"  \
                                   --virtual-ips="54.192.0.42-43"       \
                                   --watch-port=80  --create


## Under-the-hood
Under the hood, the HA configuration creates and starts up an HA sidecar
pod on all the nodes matching the given selector - can only run one HA
sidecar pod per node and the number of replicas is the number of nodes
that match. It also starts watching for any changes to nodes
(addition/deletion/modifications) and processes those change sets,
adjusting the replica count in the deployment to match the reality.

And from there on, kubernetes handles the task of ensuring there are
```n``` replicas of the HA sidecar pod.

On the nodes running the HA sidecar pod, keepalived ensures the watched
service is available and ensures that the set of VIPs is available across
the selection of nodes (or sub-cluster). This means an individual node
would be a candidate to host zero or more VIPs if the watched service
(example HAProxy on port 80) is available and 0 VIPs if the watched service
is **not** available. This also allows handling the case where the number
of VIPs is much smaller than the node selection or even nodes running the
watched service (e.g. HAProxy router) - not all nodes would be allocated a
virtual IP but would still be candidates on a failure.
And the case where the number of VIPs is more than the node selection, a
node could service multiple VIPs.

This allows us to the cover:
  1. The normal steady state case.
  1. When a cluster is resized - nodes shrink or grow.
  1. When a cluster is modified - really node labels are modified.
  1. Failure cases when a node or watched service or network fails.

Note: The PerNodeController in the future will remove the need to watch
      the nodes when a cluster is resized or modified, as the keepalived
      sidecar pod would be directly run on the given set of nodes.


## Usage
The intended usage is a workflow that follows a pattern similar to the
example shown below.

        $ #  For an HA setup, first allocate/label a pool of nodes.
        $ for i in `seq 5`; do
            openshift kube label nodes minion-$i hac=router-west
        done

        $ #  Next, enable the HA configuration on the labeled set.
        $ #  Note: This step can also be performed after starting the
        $ #        target or monitored service (in this example the
        $ #        HAProxy router below).
        $ openshift admin ha-config --credentials="${KUBECONFIG}"   \
                                    --virtual-ips=10.1.1.100-104    \
                                    --selector="hac=router-west"    \
                                    --watch-port=80 --create

        $ #  Finally, start up the router using the same selector.
        openshift admin router --credentials="${KUBECONFIG}"        \
                               --selector="hac=router-west" --create


## Exclusions
 1. Graphical User Interface (UI): This document describes what the HA
    configuration is, does and how it works under the covers. It makes
    absolutely **no** attempt to describe a graphical user interface.

