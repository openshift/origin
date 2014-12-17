# Idling OpenShift Pods
## Problem

Not all containers in an OpenShift cluster will be active at all times.  Stopping inactive
containers and creating new ones in response to demand allows for greater deployment densities.
Removing unused pods from nodes also simplifies administration.  We refer to the process of
determining which containers to stop and stopping them as "idling".  The process of creating new
pods for an idled service in response to a new request to that service is called "unidling".

## Lexicon

- **Route**: a rule linking a connection to a service 
- **Destination pods**: the pods resolve to a service; service endpoints
- **Idled service**: a service with zero pods that resolve to it
- **Idled route**: a route that points to an idled service


## Use Cases

The following use cases should be explored by this proposal:

1.  As a PaaS operator, I want destination pods for a service receiving no requests to be idled
2.  As a PaaS operator, I want a new requests that are routed to an idled service or made directly
    to the kube-proxy to trigger the creation of new destination pods for that service
3.  As a PaaS operator, I want requests to an idled route to be buffered while the service is
    being unidled

## Idling Design

A service is idle when it has no pods that resolve to it.  The autoscaler component described
in [kubernetes/2863](https://github.com/GoogleCloudPlatform/kubernetes/pull/2863) would provide the
functionality necessary to scale replication controllers down to zero in response to a lack of
requests for a certain time.

The flow of events when the autoscaler idles a service S and a replication controller R whose
pods resolve to D is as follows:

1.  The autoscaler determines that R should be scaled down to zero replicas.
2.  The autoscaler resizes R to zero
3.  The pods for R are deleted, which is reflected as Endpoint changes for S

This proposal will assume that the proposed autoscaler is available.  As the autoscaler proposal
progresses this proposal will be updated with any changes that will be required to the autoscaler.

The edge routing layer and kube-proxy layer both have to be aware of service in order to configure
themselves appropiately to unidle the service.  Actions to be taken upon idleness will be described
in the unidling design.

## Unidling Design

The requirements for unidling an idle service S with replication controller R are
as follows:

1.  The first request to S after it has become idle must:
    1.  trigger unidling of S
    2.  be buffered until a destination pod is available
2.  R must be resized to *n* >= 1
3.  Subsequent requests while to S while S is being unidled must:
    1.  cause no more unidling
    2.  be buffered until a destination pod is available

The edge routing layer and the kube-proxy layer must both implement these behaviors, and these two
layers must also effectively buffer while the unidle is occuring at the other layer.

#### A note on systemd socket activation

Our prior work on geard investigated using systemd socket activation as an unidle trigger.  The key
challenge with this approach is that it requires the interface/ports has to be defined before a pod
is started, which today is the pod being scheduled to the host. However, the pod has to be stopped
which complicates resource scheduling. In this model the scheduler cannot make resource decisions
without double checking to see if pods have been unidled, and you could potentially wake too many
pods.

### Correlating services and replication controllers

Unidling, unlike idling, requires the routing / proxy layers to be able to resolve a service to a
replication controller that manages that service's destination pods.  When a service needs to be
unidled, the components of the system that perform unidling must be able to perform this resolution
in order to know which replication controller to resize to create destination pods.

This needs to be explored further.  At this time, replication controllers and services are
intentionally decoupled.  We do not want to change the model for services or replication
controllers.

One possibility for resolving services to replication controllers would be to query the autoscaler
configuration.  The algorithm would roughly be as follows, given a service S in a namespace N:

1.  List all `AutoscalerSpec` resources in a namespace
2.  Find every `AutoscalerThreshold` that references the named service
3.  For each threshold, add each target that depends on the named service to scale to the list

The above approach is imperfect in a couple of ways:

1.  There may be multiple replication controllers that depend on a service's statistics but which
    do not actually create destination pods
2.  There may be multiple replication controllers that manage destination pods for a service; how
    should the one to resize be chosen?

### Unidling holistically considered

The in order to support unidling service from the kube-proxy layer, we know that that layer must:

1.  Track idleness of services
1.  Perform unidle in response to a request to an idled service
1.  Buffer requests to a service it is unidling

We also know that the edge routing layer must:

1.  Track idleness of services
1.  Initiate an unidle in response to a request to an idled service
1.  Allow buffering of requests to idled services.

#### Should the edge router buffer requests or always delegate?

There are many possible scenarios for how unidling functionality should be implemented across the
edge routing and kube-proxy layers.  Fundamentally they decompose into two cases: the edge
routing layer handles buffering of requests in addition to the kube-proxy layer or delegates to the
kube-proxy layer to buffer connections.  The problem with having both layers independently buffer
requests when the other has initiated the unidle is that it introduces shared state.  If the edge
routing layer delegates to the kube-proxy layer for idled services, requests to unidling services
will be buffered by the kube-proxy layer.  For this reason, we recommend that the edge routing
layer not buffer requests itself.

#### Preparing to unidle

Both the kube-proxy and the edge routing layers need to respond to service idleness to support
unidling.  The edge routing layer must remove the last endpoints from the routing table for a
service becoming idle and route future connections for that service's IP through the kube-proxy
layer.

The kube-proxy layer is more complex; it must change the socket handler for an idling service to
implement the following behavior:

1.  Resolve the idling service S to some replication controller R
2.  Resize R to *n* >= 1
3.  Block until the kube-proxy receives endpoint information for S
4.  Dispatch requests to S

#### Limits for kube-proxy number of services

Density is ultimately limited by an upper bound on the number of services that can be handled by a
single kube-proxy.  Currently that limitation is governed by the maximum number of iptables rules
that can be handled performantly, which is around 30K (**source?**).  In the future the kube-proxy
layer may be decoupled from the nodes and not be bound to any particular machines.  Non-overlapping
subnets could align to a blocks of hosts handled by different kube-proxy shards.

## Proposed Design

### Modifications to kube-proxy

### Modifications to OpenShift router
