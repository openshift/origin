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
4.  The routing layer receives a watch event with the updated state of S's endpoints
5.  The routing layer determines that S is idle
6.  The routing later takes some action to prepare to unidle S by resizing R

This proposal will assume that the proposed autoscaler is available.  As the autoscaler proposal
progresses this propsal will be updated with any changes that will be required to the autoscaler.

## Unidling Design

The requirements for unidling an idle service S with replication controller R are
as follows:

1.  The request to S after it has become idle must:
    1.  Trigger unidling of S
    2.  Be buffered until a destination pod is available
2.  R must be resized to *n* > 1
3.  Subsequent requests while to S while S is being unidled must be buffered until a
    destination pod is available

#### A note on systemd socket activation

Our prior work on geard investigated using systemd socket activation as an unidle trigger.  The key
challenge with this approach is that it requires the interface/ports has to be defined before a pod
is started, which today is the pod being scheduled to the host. However, the pod has to be stopped
which complicates resource scheduling. In this model the scheduler cannot make resource decisions
without double checking to see if pods have been unidled, and you could potentially wake too many
pods.

### Unidling from the service proxy

### Edge Unidling Design Options

#### Edge Unidling: Idle proxy embedded in router 

#### Edge Unidling: Router redirects to idle proxy; proxy uses virtual IPs to resolve service

#### Edge Unidling: Router redirects to idle proxy; proxy uses ports to resolve service

#### Edge Unidling: Router redirects to idle proxy and rewrites tcp stream to include service

## Proposed Design

### Modifications to kube-proxy

### Modifications to OpenShift router
